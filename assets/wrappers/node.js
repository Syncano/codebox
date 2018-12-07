const fs = require('fs')
const path = require('path')
const vm = require('vm')

const APP_DIR = '/app/code'
const MAX_TIMEOUT = 2147483647
const READY_STRING = 'ready'
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
let outputSeparator
let magicString
let script
let scriptFunc
let scriptContext = {
  HttpResponse,
  setResponse,
  exports,
  require,
  module,
  __filename,
  __dirname
}
let fakeTimeout
let data
let dataCursor

// Patch process.exit.
function ExitError (code) {
  this.code = code !== undefined ? code : process.exitCode
}

ExitError.prototype = new Error()
process.exit = function (code) {
  throw new ExitError(code)
}

for (let attrname in global) {
  scriptContext[attrname] = global[attrname]
  scriptContext.global = scriptContext
}

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

function renewTimeout () {
  fakeTimeout = setTimeout(renewTimeout, MAX_TIMEOUT)
}

function processScript (context) {
  process.exitCode = 0
  outputResponse = null
  outputSeparator = context._outputSeparator
  magicString = context._magicString
  const timeout = context._timeout

  // Create script and it's context if it's the first run.
  if (script === undefined) {
    const entryPoint = context._entryPoint
    const scriptFilename = path.join(APP_DIR, entryPoint)
    const source = fs.readFileSync(scriptFilename)

    scriptContext.module.filename = scriptFilename
    scriptContext.__filename = scriptFilename
    scriptContext.__dirname = path.dirname(scriptFilename)

    script = new vm.Script(source, {
      filename: entryPoint
    })
  }

  // Inject script context.
  for (let key in context) {
    if (key.startsWith('_')) {
      continue
    }
    scriptContext[key] = context[key]
  }

  if (scriptFunc === undefined) {
    // Recreate vm context if it's dirty.
    if (vm.isContext(scriptContext)) {
      let ctx = {}
      for (let key in scriptContext) {
        ctx[key] = scriptContext[key]
      }
      scriptContext = ctx
    }
    scriptContext = vm.createContext(scriptContext)

    // Start script.
    let ret = script.runInContext(scriptContext, {
      timeout: timeout / 1e6
    })
    if (typeof (ret) === 'function') {
      scriptFunc = ret
      scriptContext.__func = scriptFunc
      processScriptFunc()
    }
  } else {
    // Run script function if it's defined.
    processScriptFunc()
  }
}

function processScriptFunc () {
  SCRIPT_FUNC.runInContext(scriptContext)
}

// Unref so reading doesn't cause infinite wait.
process.stdin.unref()

// Process data on stdin.
process.stdin.on('data', function (chunk) {
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
    clearTimeout(fakeTimeout)
  }
})

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
  process.stderr.write(String.fromCharCode(code))
  if (outputResponse !== null && outputResponse instanceof HttpResponse) {
    let json = outputResponse.json()
    let buf = Buffer.allocUnsafe(4)
    buf.writeUInt32LE(json.length)

    process.stdout.write(outputSeparator)
    process.stdout.write(buf)
    process.stdout.write(json)
    process.stdout.write(outputResponse.content)
  }
  renewTimeout()
  process.stdout.write(magicString)
  process.stderr.write(magicString)
})

renewTimeout()
process.stdout.write(READY_STRING)
process.stderr.write(READY_STRING)
