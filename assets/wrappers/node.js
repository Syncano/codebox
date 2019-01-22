const fs = require('fs')
const path = require('path')
const vm = require('vm')
const net = require('net')
const os = require('os')

const APP_PATH = '/app/code'
const ENV_PATH = '/app/env'
const APP_PORT = 8123
const MUX_STDOUT = 1
const MUX_STDERR = 2
const MUX_RESPONSE = 3
const SCRIPT_FUNC = new vm.Script(`
{
  let __f

  try {
    __f = __func({
        args: ARGS,
        meta: META,
        config: CONFIG,
        HttpResponse,
        setResponse,
    })
  } catch (error) {
    __script.handleError(error)
  }

  if (__f instanceof Promise) {
    __f.catch(function (error) {
      __script.handleError(error)
    })

    __f.then(function (r) {
      __script.setResponse(r)
      __script.sendResponse()
    })

  } else {
    __script.setResponse(__f)
    __script.sendResponse()
}
}`)

let lastContext
let script
let scriptFunc
let socket
let data
let dataCursor
let origStdoutWrite
let asyncMode

// Patch process.exit.
function ExitError (code) {
  this.code = code !== undefined ? code : process.exitCode
}

ExitError.prototype = new Error()
process.exit = function (code) {
  throw new ExitError(code)
}

// Patch process stdout/stderr write.
origStdoutWrite = process.stdout.write
process.stdout.write = streamWrite(MUX_STDOUT)
process.stderr.write = streamWrite(MUX_STDERR)

let commonCtx = {
  APP_PATH,
  ENV_PATH,
  __filename,
  __dirname,

  // globals
  exports,
  process,
  Buffer,
  clearImmediate,
  clearInterval,
  clearTimeout,
  setImmediate,
  setInterval,
  setTimeout,
  console,
  module,
  require
}

// Inject globals to common context.
commonCtx['global'] = commonCtx

// HttpResponse class.
class HttpResponse {
  constructor (statusCode, content, contentType, headers) {
    this.statusCode = statusCode || 200
    this.content = content || ''
    this.contentType = contentType || 'application/json'
    this.headers = headers || {}
  }

  json () {
    for (let k in this.headers) {
      this.headers[k] = String(this.headers[k])
    }
    return JSON.stringify({
      sc: parseInt(this.statusCode),
      ct: String(this.contentType),
      h: this.headers
    })
  }
}
commonCtx['HttpResponse'] = HttpResponse

// ScriptContext class.
class ScriptContext {
  constructor (responseMux = MUX_RESPONSE) {
    this.outputResponse = null
    this.exitCode = null
    this.responseMux = responseMux
  }

  setResponse (response) {
    if (response instanceof HttpResponse) {
      this.outputResponse = response
    }
  }

  handleError (error) {
    if (error instanceof ExitError) {
      this.exitCode = error.code
      return
    }

    if (error.message === 'Script execution timed out.') {
      this.exitCode = 124
    } else {
      this.exitCode = 1
    }
    throw error
  }

  sendResponse (final = false) {
    if (!final && !asyncMode) {
      return
    }

    let exitCode = process.exitCode
    if (this.exitCode !== null) {
      exitCode = this.exitCode
    }

    if (this.outputResponse !== null && this.outputResponse instanceof HttpResponse) {
      let json = this.outputResponse.json()
      let jsonLen = Buffer.allocUnsafe(4)
      jsonLen.writeUInt32LE(Buffer.byteLength(json))
      socketWrite(socket, this.responseMux, String.fromCharCode(exitCode), jsonLen, json, this.outputResponse.content)
    } else {
      socketWrite(socket, this.responseMux, String.fromCharCode(exitCode))
    }
  }
}

// Main process script function.
function processScript (context) {
  const timeout = context._timeout

  // Create script and context if it's the first run.
  if (script === undefined) {
    // Setup async mode.
    setupAsyncMode(context._async)

    const entryPoint = context._entryPoint
    const scriptFilename = path.join(APP_PATH, entryPoint)
    const source = fs.readFileSync(scriptFilename)

    commonCtx.module.filename = scriptFilename
    commonCtx.__filename = scriptFilename
    commonCtx.__dirname = path.dirname(scriptFilename)

    script = new vm.Script(source, {
      filename: entryPoint
    })
  }

  // Prepare context.
  let ctx = Object.assign({}, commonCtx)

  for (let key in context) {
    if (key.startsWith('_')) {
      continue
    }
    ctx[key] = context[key]
  }
  ctx['__script'] = lastContext = new ScriptContext(ctx._response)
  // For backwards compatibility.
  ctx['setResponse'] = (r) => lastContext.setResponse(r)

  // Run script.
  let opts = { timeout: timeout / 1e6 }
  if (scriptFunc === undefined) {
    let ret = script.runInNewContext(ctx, opts)
    if (typeof (ret) === 'function') {
      scriptFunc = ret
      ctx['__func'] = commonCtx['__func'] = scriptFunc
      SCRIPT_FUNC.runInNewContext(ctx, opts)
    }
  } else {
    // Run script function if it's defined.
    SCRIPT_FUNC.runInNewContext(ctx, opts)
  }
}

// Create server and process data on it.
var server = net.createServer(function (sock) {
  socket = sock

  sock.on('data', function (chunk) {
    sock.unref()

    if (data === undefined) {
      let totalSize = chunk.readUInt32LE(0)
      dataCursor = 0
      data = Buffer.allocUnsafe(totalSize)
    }
    chunk.copy(data, dataCursor)
    dataCursor += chunk.length

    if (dataCursor !== data.length) {
      return
    }

    // Read context JSON.
    let contextSize = data.readUInt32LE(4)
    let contextJSON = data.slice(8, 8 + contextSize)
    let context = JSON.parse(contextJSON)

    // Process files into context.ARGS.
    if (context._files) {
      context.ARGS = context.ARGS || {}
      let cursor = 8 + contextSize

      for (let i = 0; i < context._files.length; i++) {
        let file = context._files[i]
        let buf = data.slice(cursor, cursor + file.length)
        buf.contentType = file.ct
        buf.filename = file.fname
        context.ARGS[file.name] = buf
        cursor += file.length
      }
    }
    // Clear data for next request.
    data = undefined

    try {
      processScript(context)
    } finally {
      if (!asyncMode) {
        server.unref()
      }
    }
  })

  sock.on('close', function () {
    socket = undefined
  })
})

function socketWrite (sock, mux, ...chunks) {
  if (sock === undefined) {
    return
  }

  let totalLength = 0
  for (let i = 0; i < chunks.length; i++) {
    totalLength += Buffer.byteLength(chunks[i])
  }

  if (totalLength === 0) {
    return
  }

  let header = Buffer.allocUnsafe(5)
  header.writeUInt8(mux)
  header.writeUInt32LE(totalLength, 1)
  sock.write(header)

  for (let i = 0; i < chunks.length; i++) {
    sock.write(chunks[i])
  }
}

function streamWrite (mux) {
  return function () {
    if (arguments.length > 0) {
      socketWrite(socket, mux, arguments[0])
    }
  }
}

function setupAsyncMode (async) {
  if (async === asyncMode) {
    return
  }
  asyncMode = async

  if (!asyncMode) {
    // Restart reader before exit.
    process.on('beforeExit', (code) => {
      server.ref()
      lastContext.sendResponse(true)
    })
  }
}

// Start listening.
let address = os.networkInterfaces()['eth0'][0].address
server.listen(APP_PORT, address)
server.on('listening', function () {
  origStdoutWrite.apply(process.stdout, [address + ':' + APP_PORT + '\n'])
})
