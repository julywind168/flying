local flying = {}
local service = nil

function flying.start(s)
    service = s
end

-- call by rust
function flying.on_init(core)
    setmetatable(flying, {__index = function (_, key)
        return function (first, ...)
            assert(first ~= flying, "use flying.foo() style instead of flying:foo()")
            return core[key](core, first, ...)
        end
    end})
end


local handle = {}

function handle.started()
    if service and service.started then
        service:started()
    end
end

function handle.stopping()
    if service and service.stopping then
        service:stopping()
    end
end

function handle.stopped()
    if service and service.stopped then
        service:stopped()
    end
end

function handle.request(source, session, message)
    local r = service and service.message and service:message(source, message) or ""
    if session > 0 then
        flying._respond(source, session, r)
    end
end

function handle.response(source, session, message)
    flying._wakeup(session, message)
end

function flying.on_event(event, ...)
    local f = assert(handle[event], "unknown event: " .. event)
    return f(...)
end

return flying