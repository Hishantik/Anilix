-- ani-skip.lua — mpv script to auto-skip anime intros/outros
-- Receives skip times via --script-opts: ani_skip_times=op:87.5-118.2,ed:1340.0-1370.5

local mp = require("mp")
local msg = require("mp.msg")

local skip_intervals = {}
local skipped = {}

local function parse_times()
    local opt = mp.get_opt("ani_skip_times")
    if not opt or opt == "" then
        return
    end

    for segment in opt:gmatch("[^,]+") do
        local typ, s, e = segment:match("^(%w+):([%d%.]+)-([%d%.]+)$")
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
            mp.set_property_number("time-pos", iv["end"])
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
