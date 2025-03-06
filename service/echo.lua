local flying = require "flying"

local echo = {}

function echo:started()
    -- client
    flying.fork(function()
        flying.sleep(500)
        local client = flying.socket.connect("127.0.0.1:8080")
        client:write("hello")
        flying.info("Server:", client:read(100))
        client:write("bye")
    end)

    -- server
    local listener = flying.socket.listen("127.0.0.1:8080")
    flying.info("Listen on 127.0.0.1:8080")
    while true do
        local client = listener:accept()
        local peer_addr = client:peer_addr()
        flying.info("Connected from " .. peer_addr)

        flying.fork(function()
            while true do
                local data = client:read(100)
                data = data:match("^%s*(.-)%s*$")
                if data == "" or data == "bye" then
                    flying.info("[" .. peer_addr .. "] exited")
                    return
                end
                flying.info("[" .. peer_addr .. "] " .. data)
                client:write("echo: " .. data .. "\n")
            end
        end)
    end
end

function echo:stopped()
    flying.info("echo stopped")
end

flying.start(echo)
