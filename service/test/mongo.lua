local flying = require "flying"

local test = {}

function test:started()
    flying.info("mongodb test started\n")
    local client = flying.mongodb.connect("mongodb://127.0.0.1:27017")
    local db = client:database("flying")
    local users = db:collection("users")

    users:insert_many{
        {
            name = "user_1",
            age = 18,
            hobby = {
                "eat",
                "sleep",
            }
        },
        {
            name = "user_2",
            age = 19,
            hobby = {
                "swim",
                "run",
            }
        }
    }

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

    flying.info(("insert one, name = '%s' id = '%s'"):format(name, id))
    local many_users = users:find_many({}, {sorters = {{name = -1}}})

    flying.info("the last one:")
    for key, value in pairs(many_users[1]) do
        flying.info(("%s = %s"):format(key, value))
    end
    flying.info("mongodb test end\n")
end



flying.start(test)
