local flying = require "flying"

local benchmark = {}

local function split_by_crlf(str)
    local lines = {}
    for line in str:gmatch("[^\r\n]+") do
        table.insert(lines, line)
    end
    return lines
end

function benchmark:started()
    local listener = flying.socket.listen("127.0.0.1:8899")
    print("Listen on 127.0.0.1:8899")
    print("Useage: redis-benchmark -t ping -p 8899 -c 100 -n 100000")
    while true do
        local client = listener:accept()

        flying.fork(function()
            while true do
                local data = client:read(100)
                if data == nil or data == "" then
                    client:close()
                    return
                end
                local lines = split_by_crlf(data)
                for index, line in ipairs(lines) do
                    if line == "PING" then
                        client:write("+PONG\r\n")
                    elseif line == "save" then
                        client:write("*2\r\n$4\r\nsave\r\n$23\r\n3600 1 300 100 60 10000\r\n")
                    elseif line == "appendonly" then
                        client:write("*2\r\n$10\r\nappendonly\r\n$3\r\nyes\r\n")
                    end
                end
            end
        end)
    end
end

return flying.start(benchmark)