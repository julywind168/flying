local flying = require "flying"

local bm = {}


function bm:started()
    self:start_call_benchmark()
    self:start_socket_benchmark()
end

function bm:start_socket_benchmark()
    flying.spawn("bm_socket", "service/benchmark/socket.lua")
end

function bm:start_call_benchmark()
    flying.spawn("bm_ping", "service/benchmark/ping.lua")
    local now = flying.now()
    for i = 1, 100000 do
        flying.call("bm_ping", "PING")
    end
    print(("Benchmark, 100_000 times call use: %d ms"):format(flying.now() - now))
end


flying.start(bm)