local json = require "flying.json"

local EMPTY_TABEL <const> = {}

local M = {}

local function try(s, fname, ...)
    local f = s[fname]
    if f then
        f(s, ...)
    end
    return s
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

function M.service(s)
    local state = {}
    local message = assert(s.message, "message function not found")

    function s._started(version)
        if version == 1 then
            try(state, "init")
        end
        try(state, "started", version)
    end

    function s._stopped()
        try(state, "stopped")
    end

    function s._stopping()
        try(state, "stopping")
    end

    function s._message(data)
        print("message:", data)
        return pack(message(state, table.unpack(unpack(data))))
    end

    return setmetatable(state, { __index = s })
end

return M
