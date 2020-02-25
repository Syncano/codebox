const fs = require('fs')
const path = require('path')
const vm = require('vm')
const net = require('net')
const util = require('util')

const APP_PATH = '/app/code'
const APP_LISTEN = '/tmp/wrapper.sock'
const ENV_PATH = '/app/env'
const STREAM_STDOUT = 0
const STREAM_STDERR = 1
const STREAM_RESPONSE = 2
const SCRIPT_FUNC = new vm.Script(`
{
  let __f

  try {
    __f = __func(__run)
  } catch (error) {
    __conn.handleError(error)
  }

  __f = Promise.resolve(__f)

  __f.catch(function (error) {
    __conn.handleError(error)
  })

  __f.then(function (r) {
    __run.setResponse(r)
    __conn.sendResponse()
  })
}`)

let lastContext
let script
let scriptFunc
let asyncMode
let setupDone = false
let entryPoint
let timeout

// Patch process.exit.
function ExitError (code) {
  this.code = code !== undefined ? code : process.exitCode
}

ExitError.prototype = new Error()
process.exit = function (code) {
  throw new ExitError(code)
}

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

class ConnectionContext {
  constructor (socket, delim, runCtx) {
    this.socket = socket
    this.delim = delim
    this.runCtx = runCtx
    this.exitCode = null
  }

  handleError (error) {
    if (error instanceof ExitError) {
      this.exitCode = error.code
      return
    }

    this.runCtx.error(error)

    if (error.toString().startsWith('Error: Script execution timed out')) {
      this.exitCode = 124
    } else {
      this.exitCode = 1
    }
  }

  sendResponse (final = false) {
    if (!final && !asyncMode) {
      return
    }

    // Handle exit code.
    let exitCode = process.exitCode

    if (this.exitCode !== null) {
      exitCode = this.exitCode
    }

    this.socket.write(String.fromCharCode(exitCode))

    // Handle stdout, stderr and output response.
    if (this.runCtx._stdout.length !== 0) {
      sendData(this.socket, STREAM_STDOUT, this.runCtx._stdout)
    }

    if (this.runCtx._stderr.length !== 0) {
      sendData(this.socket, STREAM_STDERR, this.runCtx._stderr)
    }

    if (this.runCtx._outputResponse !== null && this.runCtx._outputResponse instanceof HttpResponse) {
      sendData(this.socket, STREAM_RESPONSE, this.runCtx._outputResponse.json())

      let content = this.runCtx._outputResponse.content

      if (typeof (content) !== 'string' && !(content instanceof HttpResponse)) {
        content = JSON.stringify(content)
      }

      this.socket.write(content)
    }

    this.socket.end()
  }
}

class RunContext {
  constructor (args, meta, config) {
    this.HttpResponse = HttpResponse
    this.args = args
    this.meta = meta
    this.config = config

    this._outputResponse = null
    this._stdout = ''
    this._stderr = ''
  }

  setResponse (response) {
    if (response !== undefined && response instanceof HttpResponse) {
      this._outputResponse = response
    }
  }

  log (data, ...args) {
    if (asyncMode) {
      this._stdout += `${util.format(data, ...args)}\n`
    } else {
      console.log(data, ...args)
    }
  }

  error (data, ...args) {
    if (asyncMode) {
      this._stderr += `${util.format(data, ...args)}\n`
    } else {
      console.error(data, ...args)
    }
  }
}

function sendData(socket, type, data) {
    let len = Buffer.allocUnsafe(4)
    len.writeUInt32LE(Buffer.byteLength(data))

    socket.write(String.fromCharCode(type))
    socket.write(len)
    socket.write(data)
}

// Main process script function.
function processScript (socket, context) {
  // Create script and context if it's the first run.
  if (script === undefined) {
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

  const runCtx = new RunContext(ctx.ARGS, ctx.META, ctx.CONFIG)
  const connCtx = new ConnectionContext(socket, context._delim, runCtx)

  ctx['__conn'] = lastContext = connCtx
  ctx['__run'] = runCtx
  // For backwards compatibility.
  ctx['setResponse'] = (r) => runCtx.setResponse(r)

  // Run script.
  let opts = { timeout: timeout / 1e6 }

  try {
    if (scriptFunc === undefined) {
      let ret = script.runInNewContext(ctx, opts)

      if (typeof ret === 'function') {
        scriptFunc = ret
        ctx['__func'] = commonCtx['__func'] = scriptFunc

        SCRIPT_FUNC.runInNewContext(ctx, opts)
      } else if (asyncMode) {
        connCtx.sendResponse()
      }
    } else {
      // Run script function if it's defined.
      SCRIPT_FUNC.runInNewContext(ctx, opts)
    }
  } catch (error) {
    connCtx.handleError(error)

    if (asyncMode) {
      connCtx.sendResponse()
    }
  }
}

// Create server and process data on it.
function processData (socket, chunk, position = 0) {
  if (position >= chunk.length) {
    return
  }
  if (position > 0) {
    chunk = chunk.slice(position)
  }

  if (socket.buffer === undefined) {
    let totalSize = chunk.readUInt32LE(0)
    socket.offset = 0
    socket.buffer = Buffer.allocUnsafe(totalSize)
  }
  position = chunk.copy(socket.buffer, socket.offset)
  socket.offset += chunk.length

  // If we're not done reading a packet, return.
  if (socket.offset < socket.buffer.length) {
    return
  }

  if (!setupDone) {
    // Read setup JSON.
    let messageSize = socket.buffer.readUInt32LE(0)
    let messageJSON = socket.buffer.slice(4, 4 + messageSize)
    let message = JSON.parse(messageJSON)

    // Process setup package.
    entryPoint = message.entryPoint
    timeout = message.timeout
    asyncMode = message.async > 1

    // Setup non async mode hooks.
    if (!asyncMode) {
      process.on('uncaughtException', (error) => {
        lastContext.handleError(error)
      })

      // Restart reader before exit.
      process.on('beforeExit', (code) => {
        server.ref()
        lastContext.sendResponse(true)
        process.stdout.write(lastContext.delim)
        process.stderr.write(lastContext.delim)
      })
    }

    setupDone = true
  } else {
    // Read context JSON.
    let contextSize = socket.buffer.readUInt32LE(4)
    let contextJSON = socket.buffer.slice(8, 8 + contextSize)
    let context = JSON.parse(contextJSON)

    // Process files into context.ARGS.
    if (context._files) {
      context.ARGS = context.ARGS || {}
      let cursor = 8 + contextSize

      for (let i = 0; i < context._files.length; i++) {
        let file = context._files[i]
        let buf = socket.buffer.slice(cursor, cursor + file.length)
        buf.contentType = file.ct
        buf.filename = file.fname
        context.ARGS[file.name] = buf
        cursor += file.length
      }
    }

    try {
      processScript(socket, context)
    } finally {
      if (!asyncMode) {
        server.unref()
      }
    }
  }

  // Clear data for next request.
  socket.buffer = undefined
  processData(socket, chunk, position)
}

var server = net.createServer(function (socket) {
  socket.on('data', (chunk) => {
    socket.unref()
    processData(socket, chunk)
  })
})

// Start listening.
server.listen(APP_LISTEN)
server.on('listening', () => {
  console.log(APP_LISTEN)
})
