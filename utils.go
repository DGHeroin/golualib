package golualib

const LuaUtilsCode = `

function table.tostring(data)
    -- cache of tables already printed, to avoid infinite recursive loops
    local tablecache = {}
    local buffer = ""
    local padder = "    "

    local function dump(d, depth)
        local t = type(d)
        local str = tostring(d)
        if (t == "table") then
            if (tablecache[str]) then
                -- table already dumped before, so we dont
                -- dump it again, just mention it
                buffer = buffer.."<"..str..">\n"
            else
                tablecache[str] = (tablecache[str] or 0) + 1
                buffer = buffer.."("..str..") {\n"
                for k, v in pairs(d) do
                    buffer = buffer..string.rep(padder, depth + 1) .. "["..k.."] => "
                    dump(v, depth + 1)
                end
                buffer = buffer..string.rep(padder, depth) .. "}\n"
            end
        elseif (t == "number") then
            buffer = buffer.."("..t..") "..str.."\n"
        else
            buffer = buffer.."("..t..") \""..str.."\"\n"
        end
    end
    dump(data, 0)
    return buffer
end

function deepcopy(orig, copies)
    copies = copies or {}
    local orig_type = type(orig)
    local copy
    if orig_type == 'table' then
        if copies[orig] then
            copy = copies[orig]
        else
            copy = {}
            copies[orig] = copy
            for orig_key, orig_value in next, orig, nil do
                copy[deepcopy(orig_key, copies)] = deepcopy(orig_value, copies)
            end
            setmetatable(copy, deepcopy(getmetatable(orig), copies))
        end
    else -- number, string, boolean, etc
        copy = orig
    end
    return copy
end

function table.clone( tbl, to )
    return deepcopy(tbl, to)
end
function deepmerge(out, a, b, copies)
    local function fn()
        local copies = {}
        local m = deepcopy(a, copies)
        -- 检查b中, 依赖项目
        
        return m
    end
    out = out or fn()
    copies = copies or {}
    local orig_type = type(a)

    if orig_type == 'table' then
        if copies[a] then
            out = copies[a]
        elseif copies[b] then
            out = copies[b]
        else
            for k,v in pairs(b) do
                if type(v) == 'table' then
                    out[k] = {}
                    deepmerge(out[k], out[k], v)
                else
                    out[k] = v
                end
                
            end
        end
    else -- number, string, boolean, etc
        b = a
    end
    return out
end
function table.merge(a, b )
    return deepmerge(nil, a, b)
end

string.replace = function(s, pattern, repl)
    local i,j = string.find(s, pattern, 1, true)
    if i and j then
        local ret = {}
        local start = 1
        while i and j do
            table.insert(ret, string.sub(s, start, i - 1))
            table.insert(ret, repl)
            start = j + 1
            i,j = string.find(s, pattern, start, true)
        end
        table.insert(ret, string.sub(s, start))
        return table.concat(ret)
    end
    return a
end

function string.split (inputstr, sep)
    if type(inputstr) ~= 'string' then return end
    if sep == nil then
        sep = "%s"
    end
    local t = {}
    for str in string.gmatch(inputstr, "([^"..sep.."]+)") do
        table.insert(t, str)
    end
    return t
end
`
