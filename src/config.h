#ifndef USE_ZLIB
#define USE_ZLIB 1
#endif
#ifndef USE_PNG
#define USE_PNG 1
#endif
#ifndef USE_JPEG
#if defined(__MORPHOS__)
#define USE_JPEG 0
#else
#define USE_JPEG 1
#endif
#endif
#ifndef USE_LUA
#define USE_LUA 0
#endif

