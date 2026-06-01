// version.h

#if defined(__GNUC__) && !defined(__llvm__)
#if __GNUC__ > 4 || (__GNUC__ == 4 && __GNUC_MINOR__ != 6) || (__GNUC__ == 4 && __GNUC_MINOR__ == 6 && __GNUC_PATCHLEVEL__ < 2) || (__GNUC__ == 3 && __GNUC_MINOR__ < 3) || (__GNUC__ == 3 && __GNUC_MINOR__ == 3 && __GNUC_PATCHLEVEL__ < 6) || (__GNUC__ == 3 && __GNUC_MINOR__ == 4 && __GNUC_PATCHLEVEL__ < 6)
#define COMPILERVERSIONSTRINGAPPEND " (broken GCC)"
#endif
#endif

#ifndef COMPILERVERSIONSTRINGAPPEND
#define COMPILERVERSIONSTRINGAPPEND
#endif

#define	QW_VERSION              2.40
#define CLASSICQ_VERSION        "3.0.0" COMPILERVERSIONSTRINGAPPEND

#if defined(_WIN32) || defined(_WIN64)
#define QW_PLATFORM     "Windows"
#elif defined(linux)
#define QW_PLATFORM     "Linux"
#elif defined(__APPLE__)
#define QW_PLATFORM     "macOS"
#elif defined(__MORPHOS__)
#define QW_PLATFORM     "MorphOS"
#elif defined(__CYGWIN__)
#define QW_PLATFORM     "Cygwin"
#elif defined(__FreeBSD__)
#define QW_PLATFORM     "FreeBSD"
#elif defined(__NetBSD__)
#define QW_PLATFORM     "NetBSD"
#elif defined(__OpenBSD__)
#define QW_PLATFORM     "OpenBSD"
#elif defined(GEKKO)
/* Not entirely true, but close enough for now. */
#define QW_PLATFORM     "Wii"
#elif defined(AROS)
#define QW_PLATFORM     "AROS"
#endif

#if defined(__x86_64__) || defined(_M_X64)
#define QW_ARCH "amd64"
#elif defined(__aarch64__) || defined(_M_ARM64)
#define QW_ARCH "arm64"
#elif defined(__i386__) || defined(_M_IX86)
#define QW_ARCH "i386"
#elif defined(__arm__)
#define QW_ARCH "arm"
#else
#define QW_ARCH "unknown"
#endif

#ifdef GLQUAKE
#define QW_RENDERER "GL"
#else
#define QW_RENDERER "Soft"
#endif

void CL_Version_f (void);
char *VersionString (void);
