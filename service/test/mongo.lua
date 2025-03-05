local flying = require "flying"

local test = {}

function test:started()
    local client = flying.mongodb.connect("mongodb://127.0.0.1:27017")
    local db = client:database("flying")
    local users = db:collection("users")

    local name = "user_" .. tostring(flying.time()//1000)

    local id = users:insert_one({
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
    local user = users:find_one({
        name = name,
    })

    for key, value in pairs(user) do
        print(key, value)
    end
end



flying.start(test)
