local flying = require "flying"

local login = {}

function login:init()
    self.users = {}
end

function login:started()
    print("login service started")
end

function login:message(name, ...)
    print("login service received message:", name, ...)
    return true, 123
end

function login:stopping()
    print("login service stopping")
end

function login:stopped()
    print("login service stopped")
end

return flying.service(login)
