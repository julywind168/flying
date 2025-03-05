local flying = require "flying"

local echo = {}

function echo:started()
    -- client
    flying.fork(function()
        flying.sleep(500)
        local client = flying.socket.connect("127.0.0.1:8080")
        client:write("hello")
        print("Server:", client:read(100))
        client:write("bye")
    end)

    -- server
    local listener = flying.socket.listen("127.0.0.1:8080")
    print("Listen on 127.0.0.1:8080")
    while true do
        local client = listener:accept()
        local peer_addr = client:peer_addr()
        print("Connected from " .. peer_addr)

        flying.fork(function()
            while true do
                local data = client:read(100)
                data = data:match("^%s*(.-)%s*$")
                if data == "" or data == "bye" then
                    print("[" .. peer_addr .. "] exited")
                    return
                end
                print("[" .. peer_addr .. "] " .. data)
                client:write("echo: " .. data .. "\n")
            end
        end)
    end
end

function echo:stopped()
    print("echo stopped")
end

flying.start(echo)
