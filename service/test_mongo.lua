local flying = require "flying"

local test = {}

function test:started()
    local client = flying.mongodb.connect("mongodb://127.0.0.1:27017")
    local db = client:database("flying")
    local user = db:collection("user")

    local name = "user_" .. tostring(flying.time()//1000)

    local id = user:insert_one({
        name = name,
        age = 18,
        hobby = {
            "swim",
            "run",
            "eat",
            "sleep",
        }
    })

    print("insert one", name, id)
    local u = user:find_one({
        name = name,
    })

    for key, value in pairs(u) do
        print(key, value)
    end
end



flying.start(test)
