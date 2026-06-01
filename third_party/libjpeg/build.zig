const std = @import("std");

pub fn build(b: *std.Build) void {
    const upstream = b.dependency("libjpeg", .{});
    const lib = b.addLibrary(.{
        .name = "jpeg",
        .linkage = .static,
        .root_module = b.createModule(.{
            .target = b.standardTargetOptions(.{}),
            .optimize = b.standardOptimizeOption(.{}),
            .link_libc = true,
        }),
    });
    const InlineKeyword = enum { __inline__ };
    const config_header = b.addConfigHeader(.{
        .style = .{ .autoconf_undef = upstream.path("jconfig.cfg") },
        .include_path = "jconfig.h",
    }, .{
        .HAVE_PROTOTYPES = true,
        .HAVE_UNSIGNED_CHAR = true,
        .HAVE_UNSIGNED_SHORT = true,
        .void = null,
        .@"const" = null,
        .CHAR_IS_UNSIGNED = null,
        .HAVE_STDDEF_H = true,
        .HAVE_STDLIB_H = true,
        .HAVE_LOCALE_H = true,
        .NEED_BSD_STRINGS = null,
        .NEED_SYS_TYPES_H = null,
        .NEED_FAR_POINTERS = null,
        .NEED_SHORT_EXTERNAL_NAMES = null,
        .INCOMPLETE_TYPES_BROKEN = null,
        .RIGHT_SHIFT_IS_UNSIGNED = null,
        .INLINE = InlineKeyword.__inline__,
        .DEFAULT_MAX_MEM = null,
        .NO_MKTEMP = null,
        .RLE_SUPPORTED = null,
        .TWO_FILE_COMMANDLINE = null,
        .NEED_SIGNAL_CATCHER = null,
        .DONT_USE_B_MODE = null,
        .PROGRESS_REPORT = null,
    });
    lib.root_module.addConfigHeader(config_header);
    lib.installConfigHeader(config_header);
    lib.root_module.addCSourceFiles(.{ .root = upstream.path(""), .files = &libjpeg_srcs });
    lib.installHeadersDirectory(upstream.path(""), "", .{});
    b.installArtifact(lib);
}

const libjpeg_srcs = [_][]const u8{
    "jmemnobs.c",
    "jaricom.c",
    "jcomapi.c",
    "jutils.c",
    "jerror.c",
    "jmemmgr.c",
    "jcapimin.c",
    "jcapistd.c",
    "jcarith.c",
    "jctrans.c",
    "jcparam.c",
    "jdatadst.c",
    "jcinit.c",
    "jcmaster.c",
    "jcmarker.c",
    "jcmainct.c",
    "jcprepct.c",
    "jccoefct.c",
    "jccolor.c",
    "jcsample.c",
    "jchuff.c",
    "jcdctmgr.c",
    "jfdctfst.c",
    "jfdctflt.c",
    "jfdctint.c",
    "jdapimin.c",
    "jdapistd.c",
    "jdarith.c",
    "jdtrans.c",
    "jdatasrc.c",
    "jdmaster.c",
    "jdinput.c",
    "jdmarker.c",
    "jdhuff.c",
    "jdmainct.c",
    "jdcoefct.c",
    "jdpostct.c",
    "jddctmgr.c",
    "jidctfst.c",
    "jidctflt.c",
    "jidctint.c",
    "jdsample.c",
    "jdcolor.c",
    "jquant1.c",
    "jquant2.c",
    "jdmerge.c",
};
