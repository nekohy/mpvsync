-- mpvsync: start the P2P sync helper for this player instance; all logic lives in the mpvsync binary
local o = {enabled = false, peers = ""}
require("mp.options").read_options(o, "mpvsync")
if not o.enabled or o.peers == "" then return end
local win = package.config:sub(1, 1) == "\\"
local pipe = mp.get_property("input-ipc-server", "")
if pipe == "" then
    math.randomseed(os.time() + math.floor(os.clock() * 1e6))
    local id = ("mpvsync-%d-%d"):format(mp.get_property_number("pid", 0), math.random(1e9))
    pipe = win and "\\\\.\\pipe\\" .. id or "/tmp/" .. id .. ".sock"
    mp.set_property("input-ipc-server", pipe)
end
mp.command_native_async({name = "subprocess", playback_only = false, args = {
    mp.command_native({"expand-path", "~~/mpvsync" .. (win and ".exe" or "")}), "-ipc", pipe, "-peers", o.peers}}, function() end)
