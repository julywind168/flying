local flying = {}
local service = {}

function flying.start(s)
    service = s
end

local session = 0
local function newsession()
    session = session + 1
    return session
end

-- call by rust
function flying.on_init(core)
    function flying.call(dest, data)
        return core:call(dest, newsession(), data)
    end

    setmetatable(flying, {
        __index = function(_, key)
            return function(first, ...)
                assert(first ~= flying, "use flying.foo() style instead of flying:foo()")
                return core[key](core, first, ...)
            end
        end
    })
end

local handle = {}

function handle.started()
    if service.started then
        service:started()
    end
end

function handle.stopping()
    if service.stopping then
        service:stopping()
    end
end

function handle.stopped()
    function flying.call(...)
        error("cannot call flying.call in stopped state")
    end

    if service.stopped then
        service:stopped()
    end
end

function handle.request(source, message)
    return service.message and service:message(source, message) or ""
end

function flying.on_event(event, ...)
    local f = assert(handle[event], "unknown event: " .. event)
    return f(...)
end

return flying
