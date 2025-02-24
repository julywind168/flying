local flying = require "flying"

local login = {}

function login:init(ctx)
    self.users = {}
    print("login service init")
end

function login:started(ctx)
    print(("login service started, version = %d"):format(ctx.version))
end

function login:message(ctx, ...)
    print("login recv message:", ...)
    ctx.stop()
    return true, 123
end

function login:stopping(ctx)
    print("login service stopping")
end

function login:stopped(ctx)
    print("login service stopped")
end

return flying.service(login)
