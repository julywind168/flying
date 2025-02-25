local json = require "flying.json"

local EMPTY_TABEL <const> = {}
local REQUEST <const> = 0
local RESPONSE <const> = 1

local flying = {
    name = "",  -- set by rust
    version = 0,
    _session = 0,
    _wait = {},
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

function flying.resume(session, ...)
    local co = flying._wait[session]
    if co then
        flying._wait[session] = nil
        return coroutine.resume(co, ...)
    end
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

    function proxy._started(version)
        flying.version = version
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
            flying.resume(session, table.unpack(unpack(data)))
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
