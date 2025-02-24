local flying = require "flying"

local main = {}

function main:init()
    print("main service init")
    flying.newservice("service/login.lua")
end

function main:started()
    print("main service started")
end

function main:message()
end

function main:stopping()
    print("main service stopping")
end

function main:stopped()
    print("main service stopped")
end

return flying.service(main)
