local flying = require "flying"

local ping = {}

function ping:started()
end

function ping:message(source, ...)
    return "PONG"
end

function ping:stopped()
end

flying.start(ping)
