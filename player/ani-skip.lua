-- ani-skip.lua — mpv script to auto-skip anime intros/outros
-- Desktop: receives skip times via --script-opts: ani_skip_times=op=87.5-118.2#ed=1340.0-1370.5
-- Android: reads skip times from /data/data/is.xyz.mpv/files/anilix-skip.conf

local mp = require("mp")
local msg = require("mp.msg")

local skip_intervals = {}
local skipped = {}

local function read_config_file()
    local paths = {
        "/data/data/is.xyz.mpv/files/anilix-skip.conf",
        os.getenv("HOME") .. "/.anilix/anilix-skip.conf",
    }
    for _, path in ipairs(paths) do
        local f = io.open(path, "r")
        if f then
            local content = f:read("*a")
            f:close()
            if content and content ~= "" then
                return content
            end
        end
    end
    return nil
end

local function parse_times()
    -- Try script-opts first (desktop), then config file (Android)
    local opt = mp.get_opt("ani_skip_times")
    if not opt or opt == "" then
        opt = read_config_file()
    end
    if not opt or opt == "" then
        return
    end

    for segment in opt:gmatch("[^#]+") do
        local typ, s, e = segment:match("^(%w+)=([%d%.]+)-([%d%.]+)$")
        if typ and s and e then
            table.insert(skip_intervals, {
                type = typ,
                start = tonumber(s),
                ["end"] = tonumber(e),
            })
        end
    end

    if #skip_intervals > 0 then
        msg.info("loaded " .. #skip_intervals .. " skip interval(s)")
    end
end

local function on_time_pos(_, pos)
    if not pos then return end

    for i, iv in ipairs(skip_intervals) do
        if not skipped[i] and pos >= iv.start and pos < iv["end"] then
            skipped[i] = true
            local label = iv.type == "op" and "intro" or "outro"
            mp.osd_message("Skipped " .. label, 1.5)
            msg.info("skipping " .. label .. " (" .. iv.start .. " -> " .. iv["end"] .. ")")
            mp.commandv("seek", iv["end"], "absolute", "exact")
            return
        end
    end
end

local function on_file_loaded()
    skip_intervals = {}
    skipped = {}
    parse_times()
end

mp.register_event("file-loaded", on_file_loaded)
mp.observe_property("time-pos", "number", on_time_pos)
