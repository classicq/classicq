package engine

// lua.c is compiled with USE_LUA=0, which leaves only these empty hooks

func Lua_Frame() {
}

func Lua_MessageFunctions(level int, message string) {
}

func Lua_Key(key int) {
}

func Lua_Frame_2D() {
}

func Lua_Init() {
}

func Lua_Shutdown() {
}

func Lua_CvarInit() {
}
