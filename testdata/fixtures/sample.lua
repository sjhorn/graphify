local http = require("http")
local json = require("json")

local Config = {
    baseUrl = "https://api.example.com",
    timeout = 30
}

local HttpClient = {}
HttpClient.__index = HttpClient

function HttpClient:new(config)
    local self = setmetatable({}, HttpClient)
    self.config = config
    return self
end

function HttpClient:get(path)
    return self:buildRequest("GET", path)
end

function HttpClient:post(path, body)
    return self:buildRequest("POST", path)
end

function HttpClient:buildRequest(method, path)
    return method .. " " .. self.config.baseUrl .. path
end

local function createClient(baseUrl)
    local config = { baseUrl = baseUrl, timeout = 30 }
    return HttpClient:new(config)
end

return {
    HttpClient = HttpClient,
    createClient = createClient
}
