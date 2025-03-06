local flying = require "flying"

local main = {}

function main:started()
    flying.info("main started")
    flying.spawn("echo", "service/echo.lua")

    -- flying.spawn("test_mongo", "service/test/mongo.lua");
    -- flying.spawn("benchmark", "service/benchmark/init.lua")
end


flying.start(main)
