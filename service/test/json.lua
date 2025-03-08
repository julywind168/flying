local flying = require "flying"
local json = flying.json

local test = {}

function test:started()
    local t = {hello = "world", array = {1, 2, 3}}
    local s = json.encode(t)
    flying.info("json string: " .. s)
    local t2 = json.decode(s)
    flying.info("json object: " .. json.encode_pretty(t2))
end

flying.start(test)
