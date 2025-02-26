local json = require "flying.json"

local EMPTY_TABEL <const> = {}
local REQUEST <const> = 0
local RESPONSE <const> = 1

local flying = {
    name = "", -- set by rust
    version = 0,
    _session = 0,
    _wait = {},
    _sleep = {},
}

function flying.session()
    flying._session = flying._session + 1
    return flying._session
end

local function pack(...)
    return json.encode({ ... })
end

local function unpack(data)
    if data == "" then
        return EMPTY_TABEL
    end
    return json.decode(data)
end

function flying.wait(session)
    local co = coroutine.running()
    flying._wait[session] = co
    return coroutine.yield()
end

function flying.wakeup(session, ...)
    local co = flying._wait[session]
    if co then
        flying._wait[session] = nil
        return coroutine.resume(co, ...)
    end
end

function flying.sleep(ms)
    local time = flying.time() + ms
    table.insert(flying._sleep, { time = time, co = coroutine.running() })
    table.sort(flying._sleep, function(a, b)
        return a.time < b.time
    end)
    flying.set_next_tick_time(flying._sleep[1].time)
    return coroutine.yield()
end

function flying.tick()
    local now = flying.time()
    while #flying._sleep > 0 and flying._sleep[1].time <= now do
        local item = table.remove(flying._sleep, 1)
        coroutine.resume(item.co)
    end
    flying.set_next_tick_time(flying._sleep[1] and flying._sleep[1].time or nil)
end

function flying.call(source, ...)
    local data = pack(...)
    local session = flying.session()
    flying.send_message(source, session, REQUEST, data)
    return flying.wait(session)
end

function flying.send(source, ...)
    local data = pack(...)
    flying.send_message(source, 0, REQUEST, data) -- session 0 means no need response
end

function flying.service(serv)
    local state = {}
    local proxy = {}
    assert(serv.message, "message function not found")

    local function task(f, ...)
        local ok, err = coroutine.resume(coroutine.create(f), ...)
        if not ok then
            print(err)
        end
    end

    function proxy._tick()
        flying.tick()
    end

    function proxy._started(version)
        flying.version = version
        -- todo: set_next_tick_duration
        if (flying.version == 1 and serv.init) or serv.started then
            task(function()
                if serv.init and flying.version == 1 then
                    serv.init(state)
                end
                if serv.started then
                    serv.started(state)
                end
            end)
        end
    end

    function proxy._stopped()
        if serv.stopped then
            task(serv.stopped, state)
        end
    end

    function proxy._stopping()
        if serv.stopping then
            task(serv.stopping, state)
        end
    end

    function proxy._message(source, session, type, data)
        -- print("recv message", source, session, type, data)
        if type == REQUEST then
            task(function()
                local res = pack(serv.message(state, table.unpack(unpack(data))))
                if session > 0 then
                    flying.send_message(source, session, RESPONSE, res)
                end
            end)
        else
            flying.wakeup(session, table.unpack(unpack(data)))
        end
    end

    return proxy
end

function flying.oneshotservice(init)
    local serv = {}

    function serv:init()
        init()
        flying.stop()
    end

    function serv:message()
    end

    return flying.service(serv)
end

return flying
