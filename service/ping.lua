local flying = require "flying"

local ping = {}

function ping:init(ctx)
    print("ping service init")
end

function ping:started(ctx)
    print(("ping service started, version = %d"):format(ctx.version))
    print("hello", flying.getenv("hello"))
end

function ping:message(ctx, ...)
    print("ping recv message:", ...)
    ctx.stop()
    return true, 123
end

function ping:stopping(ctx)
    print("ping service stopping")
end

function ping:stopped(ctx)
    print("ping service stopped")
end

return flying.service(ping)
