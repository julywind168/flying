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

local function log_msg(...)
    local list = {...}
    for index, value in ipairs(list) do
        if type(value) == "table" then
            local str = flying.json.encode(value)
            list[index] = #str <= 100 and str or flying.json.encode_pretty(value)
        else
            list[index] = tostring(value)
        end
    end
    table.insert(list, 1, ("[%s]"):format(flying.name()))
    return table.concat(list, " ")
end

-- call by rust
function flying.on_init(core)
    function flying.call(dest, data)
        return core:call(dest, newsession(), data)
    end
    function flying.error(...)
        core:error(log_msg(...))
    end
    function flying.warn(...)
        core:warn(log_msg(...))
    end
    function flying.info(...)
        core:info(log_msg(...))
    end
    function flying.debug(...)
        core:debug(log_msg(...))
    end
    function flying.trace(...)
        core:trace(log_msg(...))
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
