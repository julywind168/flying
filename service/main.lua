local flying = require "flying"

local function main()
    flying.setenv("hello", "world")
    flying.newservice("service/login.lua")
end

return flying.oneshotservice(main)
