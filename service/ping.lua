local flying = require "flying"

local ping = {}

function ping:started()
    print("ping started")
end

function ping:message(source, message)
    print("ping message", source, message)
    return "PONG"
end

function ping:stopped()
    print("ping stopped")
end

flying.start(ping)
