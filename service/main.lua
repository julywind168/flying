local flying = require "flying"

local function main()
    flying.setenv("hello", "world")
    flying.newservice("ping", "service/ping.lua")
    flying.send_message("ping", 0, 0, '["hello world from main"]')
end

return flying.oneshotservice(main)
