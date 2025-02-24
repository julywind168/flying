local json = require "flying.json"

local EMPTY_TABEL <const> = {}

local flying = {}

local function try(serv, fname, state, ctx, ...)
    local f = serv[fname]
    if f then
        f(state, ctx, ...)
    end
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

function flying.service(serv)
    local state = {}
    local context = {}
    assert(serv.message, "message function not found")

    function context._started(version)
        context.version = version
        if version == 1 then
            try(serv, "init", state, context)
        end
        try(serv, "started", state, context)
    end

    function context._stopped()
        try(serv, "stopped", state, context)
    end

    function context._stopping()
        try(serv, "stopping", state, context)
    end

    function context._message(data)
        return pack(serv.message(state, context, table.unpack(unpack(data))))
    end

    return context
end

function flying.oneshotservice(init)
    local serv = {}

    function serv:init(ctx)
        init(ctx)
        ctx.stop()
    end

    function serv:message()
    end

    return flying.service(serv)
end

return flying
