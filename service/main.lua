local flying = require "flying"

local function main()
    flying.setenv("hello", "world")
    flying.newservice("ping", "service/ping.lua")
    flying.send("ping", "How are you?")
    local pong = flying.call("ping", "ping")
    print(pong)
end

return flying.oneshotservice(main)
