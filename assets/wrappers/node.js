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
  let __f = __func({
      args: ARGS,
      meta: META,
      config: CONFIG,
      HttpResponse,
      setResponse,
  })
  if (__f instanceof Promise) {
    __f.then(function (r) {
      if (r instanceof HttpResponse) {
        setResponse(r)
      }
    })
  } else if (__f instanceof HttpResponse) {
    setResponse(__f)
  }
}`)

let outputResponse
let socket
let script
let scriptFunc
let scriptContext = {
  APP_PATH,
  ENV_PATH,
  HttpResponse,
  setResponse,
  exports,
  require,
  module,
  __filename,
  __dirname
}
let data
let dataCursor
let origStdoutWrite

// Patch process.exit.
function ExitError (code) {
  this.code = code !== undefined ? code : process.exitCode
}

ExitError.prototype = new Error()
process.exit = function (code) {
  throw new ExitError(code)
}

// Inject globals to script context.
for (let attrname in global) {
  scriptContext[attrname] = global[attrname]
  scriptContext['global'] = scriptContext
}

// HttpResponse class.
function HttpResponse (statusCode, content, contentType, headers) {
  this.statusCode = statusCode || 200
  this.content = content || ''
  this.contentType = contentType || 'application/json'
  this.headers = headers || {}
}

HttpResponse.prototype.json = function () {
  for (let k in this.headers) {
    this.headers[k] = String(this.headers[k])
  }
  return JSON.stringify({
    sc: parseInt(this.statusCode),
    ct: String(this.contentType),
    h: this.headers
  })
}

function setResponse (response) {
  outputResponse = response
}

// Main process script function.
function processScript (context) {
  const timeout = context._timeout
  let ctx = {}
  process.exitCode = 0
  outputResponse = null

  // Create script and context if it's the first run.
  if (script === undefined) {
    const entryPoint = context._entryPoint
    const scriptFilename = path.join(APP_PATH, entryPoint)
    const source = fs.readFileSync(scriptFilename)

    scriptContext.module.filename = scriptFilename
    scriptContext.__filename = scriptFilename
    scriptContext.__dirname = path.dirname(scriptFilename)

    script = new vm.Script(source, {
      filename: entryPoint
    })
  }

  // Prepare context.
  for (let key in scriptContext) {
    ctx[key] = scriptContext[key]
  }

  for (let key in context) {
    if (key.startsWith('_')) {
      continue
    }
    ctx[key] = context[key]
  }

  if (scriptFunc === undefined) {
    // Start script.
    let ret = script.runInNewContext(ctx, {
      timeout: timeout / 1e6
    })
    if (typeof (ret) === 'function') {
      scriptFunc = ret
      ctx['__func'] = scriptContext['__func'] = scriptFunc
      processScriptFunc(ctx)
    }
  } else {
    // Run script function if it's defined.
    processScriptFunc(ctx)
  }
}

function processScriptFunc (ctx) {
  SCRIPT_FUNC.runInNewContext(ctx)
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
    let context
    let contextJSON = data.slice(8, 8 + contextSize)
    context = JSON.parse(contextJSON)

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
      server.unref()
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

// Patch process.stderr and stdout.
origStdoutWrite = process.stdout.write
process.stdout.write = function () {
  if (arguments.length > 0) {
    socketWrite(socket, MUX_STDOUT, arguments[0])
  }
}
process.stderr.write = function () {
  if (arguments.length > 0) {
    socketWrite(socket, MUX_STDERR, arguments[0])
  }
}

// Process uncaught exception.
process.on('uncaughtException', (e) => {
  if (e instanceof ExitError) {
    process.exitCode = e.code
    return
  }

  console.error(e)
  if (e.message === 'Script execution timed out.') {
    process.exitCode = 124
  } else {
    process.exitCode = 1
  }
})

// Restart reader before exit.
process.on('beforeExit', (code) => {
  server.ref()

  if (outputResponse !== null && outputResponse instanceof HttpResponse) {
    let json = outputResponse.json()
    let jsonLen = Buffer.allocUnsafe(4)
    jsonLen.writeUInt32LE(Buffer.byteLength(json))
    socketWrite(socket, MUX_RESPONSE, String.fromCharCode(code), jsonLen, json, outputResponse.content)
  } else {
    socketWrite(socket, MUX_RESPONSE, String.fromCharCode(code))
  }
})

// Start listening.
let address = os.networkInterfaces()['eth0'][0].address
server.listen(APP_PORT, address)
server.on('listening', function () {
  origStdoutWrite.apply(process.stdout, [address + ':' + APP_PORT + '\n'])
})
