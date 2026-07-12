/*
Copyright (C) 2026 classicQ contributors

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.
*/

#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <string.h>
#include <time.h>

#define SDL_MAIN_HANDLED 1
#include <SDL3/SDL.h>
#include <SDL3/SDL_main.h>

#include "quakedef.h"

#if defined(_WIN32)
#  include <windows.h>
#  include <bcrypt.h>
#  include <shlobj.h>
#else
#  include <unistd.h>
#  include <fcntl.h>
#  include <sys/time.h>
#endif

#if defined(__APPLE__)
#  include <mach-o/dyld.h>
#endif

#if defined(_WIN32)
#  define USERDATA_SUBDIR "\\classicQ"
#elif defined(__APPLE__)
#  define USERDATA_SUBDIR "/Library/Application Support/classicQ"
#else
#  define USERDATA_SUBDIR "/.classicq"
#endif

void Sys_Printf(char *fmt, ...)
{
	va_list ap;
	va_start(ap, fmt);
	vfprintf(stdout, fmt, ap);
	va_end(ap);
}

void Sys_Error(char *error, ...)
{
	static int in_error;
	char text[1024];
	va_list ap;

	va_start(ap, error);
	vsnprintf(text, sizeof(text), error, ap);
	va_end(ap);

	fprintf(stderr, "FATAL: %s\n", text);

	if (!in_error)
	{
		in_error = 1;
		Host_Shutdown();
		SDL_ShowSimpleMessageBox(SDL_MESSAGEBOX_ERROR, "classicQ fatal error", text, NULL);
	}

	exit(1);
}

void Sys_Quit(void)
{
	Host_Shutdown();
	exit(0);
}

void Sys_Init(void)
{
}

void Sys_CvarInit(void)
{
}

void Sys_MicroSleep(unsigned int microseconds)
{
#if defined(_WIN32)
	Sleep(microseconds / 1000);
#else
	struct timespec ts;
	ts.tv_sec = microseconds / 1000000;
	ts.tv_nsec = (microseconds % 1000000) * 1000;
	nanosleep(&ts, NULL);
#endif
}

void Sys_SleepTime(unsigned int usec)
{
	Sys_MicroSleep(usec);
}

double Sys_DoubleTime(void)
{
#if defined(_WIN32)
	static LARGE_INTEGER freq;
	static int initialized = 0;
	LARGE_INTEGER now;
	if (!initialized)
	{
		QueryPerformanceFrequency(&freq);
		initialized = 1;
	}
	QueryPerformanceCounter(&now);
	return (double)now.QuadPart / (double)freq.QuadPart;
#else
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (double)ts.tv_sec + (double)ts.tv_nsec * 1e-9;
#endif
}

unsigned long long Sys_IntTime(void)
{
#if defined(_WIN32)
	static LARGE_INTEGER freq;
	static int initialized = 0;
	LARGE_INTEGER now;
	if (!initialized)
	{
		QueryPerformanceFrequency(&freq);
		initialized = 1;
	}
	QueryPerformanceCounter(&now);
	return (unsigned long long)(now.QuadPart * 1000000ULL / freq.QuadPart);
#else
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (unsigned long long)ts.tv_sec * 1000000ULL + (unsigned long long)(ts.tv_nsec / 1000);
#endif
}

char *Sys_ConsoleInput(void)
{
	return 0;
}

void Sys_MakeCodeWriteable(unsigned long startaddr, unsigned long length)
{
	(void)startaddr;
	(void)length;
}

void Sys_RandomBytes(void *target, unsigned int numbytes)
{
#if defined(_WIN32)
	BCryptGenRandom(NULL, (PUCHAR)target, (ULONG)numbytes, BCRYPT_USE_SYSTEM_PREFERRED_RNG);
#else
	int fd = open("/dev/urandom", O_RDONLY);
	if (fd >= 0)
	{
		unsigned char *p = target;
		while (numbytes)
		{
			ssize_t n = read(fd, p, numbytes);
			if (n <= 0) break;
			p += n;
			numbytes -= (unsigned int)n;
		}
		close(fd);
	}
#endif
}

void Sys_LowFPPrecision(void) {}
void Sys_HighFPPrecision(void) {}
void Sys_SetFPCW(void) {}

const char *Sys_GetRODataPath(void)
{
#if defined(_WIN32)
	char path[MAX_PATH];
	DWORD r = GetModuleFileNameA(NULL, path, sizeof(path));
	if (r == 0 || r >= sizeof(path))
		return NULL;
	char *p = strrchr(path, '\\');
	if (!p)
		return NULL;
	*p = 0;
	char *ret = malloc(strlen(path) + 1);
	if (ret)
		strcpy(ret, path);
	return ret;
#elif defined(__APPLE__)
	char path[4096];
	uint32_t size = sizeof(path);
	if (_NSGetExecutablePath(path, &size) != 0)
		return NULL;
	char *p = strrchr(path, '/');
	if (!p)
		return NULL;
	*p = 0;
	const char *bundle_suffix = "/Contents/MacOS";
	size_t pathlen = strlen(path);
	size_t suflen = strlen(bundle_suffix);
	if (pathlen > suflen && memcmp(path + pathlen - suflen, bundle_suffix, suflen) == 0)
	{
		path[pathlen - suflen] = 0;
		char *slash = strrchr(path, '/');
		if (slash)
			*slash = 0;
	}
	char *ret = malloc(strlen(path) + 1);
	if (ret)
		strcpy(ret, path);
	return ret;
#elif defined(__linux__)
	char path[4096];
	ssize_t len = readlink("/proc/self/exe", path, sizeof(path) - 1);
	if (len <= 0)
		return NULL;
	path[len] = 0;
	char *p = strrchr(path, '/');
	if (!p)
		return NULL;
	*p = 0;
	char *ret = malloc(strlen(path) + 1);
	if (ret)
		strcpy(ret, path);
	return ret;
#else
	return NULL;
#endif
}

const char *Sys_GetUserDataPath(void)
{
	const char *env = getenv("CLASSICQ_USERDATA");
	if (env && *env)
	{
		char *copy = malloc(strlen(env) + 1);
		if (copy)
			strcpy(copy, env);
		return copy;
	}

#if defined(_WIN32)
	char base[MAX_PATH];
	if (SHGetFolderPathA(NULL, CSIDL_APPDATA | CSIDL_FLAG_CREATE, NULL, 0, base) != S_OK)
		return NULL;
	size_t n = strlen(base) + strlen(USERDATA_SUBDIR) + 1;
	char *ret = malloc(n);
	if (ret)
		snprintf(ret, n, "%s%s", base, USERDATA_SUBDIR);
	return ret;
#else
	const char *home = getenv("HOME");
	if (!home || !*home)
		return NULL;
	size_t n = strlen(home) + strlen(USERDATA_SUBDIR) + 1;
	char *ret = malloc(n);
	if (ret)
		snprintf(ret, n, "%s%s", home, USERDATA_SUBDIR);
	return ret;
#endif
}

const char *Sys_GetLegacyDataPath(void)
{
	char buf[1024];
#if defined(_WIN32)
	if (GetCurrentDirectoryA(sizeof(buf), buf) == 0)
		return NULL;
#else
	if (!getcwd(buf, sizeof(buf)))
		return NULL;
#endif
	char *ret = malloc(strlen(buf) + 1);
	if (ret)
		strcpy(ret, buf);
	return ret;
}

void Sys_FreePathString(const char *p)
{
	free((void *)p);
}

#if defined(_WIN32)
#include <tlhelp32.h>
#include <dbghelp.h>

static void Sys_CrashPrintAddr(FILE *f, HANDLE proc, DWORD64 addr)
{
	char symbuf[sizeof(SYMBOL_INFO) + 256];
	SYMBOL_INFO *sym = (SYMBOL_INFO *)symbuf;
	IMAGEHLP_LINE64 line;
	DWORD64 disp64 = 0;
	DWORD disp32 = 0;
	char *base = (char *)GetModuleHandleA(NULL);

	fprintf(f, "%p (exe offset 0x%llx)", (void *)addr,
		(unsigned long long)((char *)addr - base));

	memset(symbuf, 0, sizeof(symbuf));
	sym->SizeOfStruct = sizeof(SYMBOL_INFO);
	sym->MaxNameLen = 255;
	if (SymFromAddr(proc, addr, &disp64, sym))
		fprintf(f, " %s+0x%llx", sym->Name, (unsigned long long)disp64);

	memset(&line, 0, sizeof(line));
	line.SizeOfStruct = sizeof(line);
	if (SymGetLineFromAddr64(proc, addr, &disp32, &line))
		fprintf(f, " [%s:%lu]", line.FileName, line.LineNumber);

	fprintf(f, "\n");
}

// writes crash.txt next to the executable: symbolized stack and module map
static LONG WINAPI Sys_CrashHandler(EXCEPTION_POINTERS *info)
{
	FILE *f;
	HANDLE snap;
	HANDLE proc = GetCurrentProcess();
	MODULEENTRY32 me;
	CONTEXT ctx;
	STACKFRAME64 frame;
	int i;
	void *addr = info->ExceptionRecord->ExceptionAddress;
	char *base = (char *)GetModuleHandleA(NULL);
	char path[MAX_PATH];
	DWORD n;

	n = GetModuleFileNameA(NULL, path, sizeof(path) - 16);
	while (n && path[n - 1] != '\\' && path[n - 1] != '/')
		n--;
	strcpy(path + n, "crash.txt");

	f = fopen(path, "w");
	if (!f)
		return EXCEPTION_CONTINUE_SEARCH;

	SymSetOptions(SYMOPT_LOAD_LINES | SYMOPT_UNDNAME);
	SymInitialize(proc, NULL, TRUE);

	fprintf(f, "exception 0x%08lx at ", info->ExceptionRecord->ExceptionCode);
	Sys_CrashPrintAddr(f, proc, (DWORD64)addr);
	fprintf(f, "exe base %p\n\nstack:\n", base);

	ctx = *info->ContextRecord;
	memset(&frame, 0, sizeof(frame));
	frame.AddrPC.Offset = ctx.Rip;
	frame.AddrPC.Mode = AddrModeFlat;
	frame.AddrFrame.Offset = ctx.Rbp;
	frame.AddrFrame.Mode = AddrModeFlat;
	frame.AddrStack.Offset = ctx.Rsp;
	frame.AddrStack.Mode = AddrModeFlat;

	for (i = 0; i < 32; i++)
	{
		if (!StackWalk64(IMAGE_FILE_MACHINE_AMD64, proc, GetCurrentThread(),
			&frame, &ctx, NULL, SymFunctionTableAccess64, SymGetModuleBase64, NULL))
			break;
		if (!frame.AddrPC.Offset)
			break;
		fprintf(f, "%2d: ", i);
		Sys_CrashPrintAddr(f, proc, frame.AddrPC.Offset);
	}

	snap = CreateToolhelp32Snapshot(TH32CS_SNAPMODULE, 0);
	if (snap != INVALID_HANDLE_VALUE)
	{
		fprintf(f, "\nmodules:\n");
		me.dwSize = sizeof(me);
		if (Module32First(snap, &me))
		{
			do
			{
				fprintf(f, "%p - %p %s\n", (void *)me.modBaseAddr,
					(void *)(me.modBaseAddr + me.modBaseSize), me.szModule);
			} while (Module32Next(snap, &me));
		}
		CloseHandle(snap);
	}

	fclose(f);
	return EXCEPTION_CONTINUE_SEARCH;
}
#endif

int main(int argc, char **argv)
{
	double newtime, oldtime;

#if defined(_WIN32)
	SetUnhandledExceptionFilter(Sys_CrashHandler);
#endif

	SDL_SetMainReady();

	if (!SDL_Init(0))
	{
		fprintf(stderr, "SDL_Init failed: %s\n", SDL_GetError());
		return 1;
	}

	COM_InitArgv(argc, argv);

	Host_Init(argc, argv);

	oldtime = Sys_DoubleTime();
	for (;;)
	{
		newtime = Sys_DoubleTime();
		Host_Frame(newtime - oldtime);
		oldtime = newtime;
	}

	return 0;
}

#if defined(_WIN32)
int WINAPI WinMain(HINSTANCE hInst, HINSTANCE hPrev, LPSTR lpCmdLine, int nShow)
{
	(void)hInst; (void)hPrev; (void)lpCmdLine; (void)nShow;
	return main(__argc, __argv);
}
#endif
