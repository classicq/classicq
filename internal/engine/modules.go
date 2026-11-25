package engine

import "runtime"

// modules.c: registry of the optional image libraries. PNG comes from the
// standard library, so the registry only says what is available.

var QLIB_LIBRARY_EXTENSION = map[bool]string{true: ".dll", false: ".so"}[runtime.GOOS == "windows"]

type qlib_id_t int

const (
	qlib_libpng qlib_id_t = iota
	qlib_libjpeg
	qlib_nummodules
)

type qlib_shutdown_fn func()

const QLIB_ERROR_MODULE_MISSING_PROC = -2

type registeredModule_t struct {
	id       qlib_id_t
	loaded   bool
	shutdown qlib_shutdown_fn
}

var registeredModules [qlib_nummodules]registeredModule_t

func QLib_Init() {
	for i := qlib_id_t(0); i < qlib_nummodules; i++ {
		registeredModules[i].id = i
		registeredModules[i].loaded = false
		registeredModules[i].shutdown = nil
	}
}

func QLib_Shutdown() {
	for i := qlib_id_t(0); i < qlib_nummodules; i++ {
		if registeredModules[i].loaded {
			registeredModules[i].shutdown()
			registeredModules[i].loaded = false
		}
	}
}

func QLib_RegisterModule(module qlib_id_t, shutdown qlib_shutdown_fn) {
	if module < 0 || module >= qlib_nummodules {
		Sys_Error("QLib_isModuleLoaded: bad module %d", module)
	}

	registeredModules[module].loaded = true
	registeredModules[module].shutdown = shutdown
}

func QLib_isModuleLoaded(module qlib_id_t) bool {
	if module < 0 || module >= qlib_nummodules {
		Sys_Error("QLib_isModuleLoaded: bad module %d", module)
	}

	return registeredModules[module].loaded
}

// QLib_ProcessProcdef resolves symbols of a loaded shared library; nothing
// is loaded dynamically here.

func QLib_MissingModuleError(errortype int, libname, cmdline, features string) {
	switch errortype {
	case QLIB_ERROR_MODULE_MISSING_PROC:
		Sys_Error("Broken \"%s"+QLIB_LIBRARY_EXTENSION+"\" library - required function missing.", libname)
	default:
		Sys_Error("QLib_MissingModuleError: unknown error type (%d)", errortype)
	}
}
