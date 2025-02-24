local flying = require "flying"

local function main()
    flying.newservice("service/login.lua")
end

return flying.oneshotservice(main)
