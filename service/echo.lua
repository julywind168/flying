local flying = require "flying"

local echo = {}

function echo:started()
    local listener = flying.socket.listen("127.0.0.1:8080")
    print("listening on 127.0.0.1:8080")


    flying.fork(function ()
        flying.sleep(2000)
        local client = flying.socket.connect("127.0.0.1:8080")
        client:write("hello")
        print("recv:", client:read(100))
        client:close()
    end)

    while true do
        local client = listener:accept()
        local peer_addr = client:peer_addr()
        print("connected from " .. peer_addr)

        flying.fork(function()
            while true do
                local data = client:read(100)
                data = data:match("^%s*(.-)%s*$")
                print("[" .. peer_addr .. "] " .. data)
                if data == "" or data == "bye" then
                    client:write("bye bye\n")
                    client:close()
                    print("[" .. peer_addr .. "] exited")
                    return
                end
                client:write("echo: " .. data .. "\n")
            end
        end)
    end
end

function echo:stopped()
    print("echo stopped")
end

flying.start(echo)
