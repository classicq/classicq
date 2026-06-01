#if defined(BUILD_STRL) && !defined(__APPLE__)
#include <sys/types.h>

size_t strlcpy(char *dst, const char *src, size_t size);
size_t strlcat(char *dst, const char *src, size_t siz);
#endif

