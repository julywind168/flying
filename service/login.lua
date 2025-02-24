local flying = require "flying"

local login = {}

function login:init()
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

function login:stopping()
    print("login service stopping")
end

function login:stopped()
    print("login service stopped")
end

return flying.service(login)
