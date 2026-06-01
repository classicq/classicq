// Build script for classicQ.
//
// zig build

const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{ .preferred_optimize_mode = .ReleaseFast });

    const root_mod = b.createModule(.{
        .target = target,
        .optimize = optimize,
        .link_libc = true,
    });

    const exe = b.addExecutable(.{
        .name = "classicq",
        .root_module = root_mod,
    });

    const sdl_dep = b.dependency("SDL", .{
        .target = target,
        .optimize = optimize,
    });
    root_mod.linkLibrary(sdl_dep.artifact("SDL2"));
    root_mod.addIncludePath(sdl_dep.path("include"));
    root_mod.addIncludePath(sdl_dep.path("include-pregen"));

    const zlib_dep = b.dependency("zlib", .{
        .target = target,
        .optimize = optimize,
    });
    root_mod.linkLibrary(zlib_dep.artifact("z"));

    const libpng_dep = b.dependency("libpng", .{
        .target = target,
        .optimize = optimize,
    });
    root_mod.linkLibrary(libpng_dep.artifact("png"));

    const libjpeg_dep = b.dependency("libjpeg", .{
        .target = target,
        .optimize = optimize,
    });
    root_mod.linkLibrary(libjpeg_dep.artifact("jpeg"));

    const c_flags = [_][]const u8{
        "-std=c23",
        "-DCLIENTONLY",
        "-DNETQW",
        "-DGLQUAKE",
        "-DUSE_PNG=1",
        "-DUSE_JPEG=1",
        "-DUSE_ZLIB=1",
        "-DUSE_LUA=0",
        "-DBUILD_STRL",
        "-fno-strict-aliasing",
        "-fcommon",
        "-fno-sanitize=undefined",
        "-Wno-int-conversion",
        "-Wno-incompatible-pointer-types",
        "-Wno-pointer-sign",
        "-Wno-switch",
        "-Wno-#warnings",
        "-Wno-date-time",
    };

    root_mod.addIncludePath(b.path("src"));

    root_mod.addCSourceFiles(.{
        .root = b.path("src"),
        .files = &common_sources,
        .flags = &c_flags,
    });

    root_mod.addCSourceFiles(.{
        .root = b.path("src"),
        .files = &sdl_sources,
        .flags = &c_flags,
    });

    const os_tag = target.result.os.tag;
    switch (os_tag) {
        .windows => {
            root_mod.addCSourceFiles(.{
                .root = b.path("src"),
                .files = &win_sources,
                .flags = &c_flags,
            });
            root_mod.linkSystemLibrary("opengl32", .{});
            root_mod.linkSystemLibrary("ws2_32", .{});
            root_mod.linkSystemLibrary("winmm", .{});
            root_mod.linkSystemLibrary("gdi32", .{});
            root_mod.linkSystemLibrary("bcrypt", .{});
            // app icon: RC writes basename, include path resolves at compile time
            root_mod.addWin32ResourceFile(.{
                .file = b.path("src/classicq.rc"),
                .include_paths = &.{b.path("assets/icons")},
            });
            // WinMain lives in sys_sdl.c; no background console
            exe.subsystem = .Windows;
        },
        .linux => {
            root_mod.addCSourceFiles(.{
                .root = b.path("src"),
                .files = &posix_sources,
                .flags = &c_flags,
            });
            root_mod.linkSystemLibrary("GL", .{});
            root_mod.linkSystemLibrary("pthread", .{});
            root_mod.linkSystemLibrary("dl", .{});
            root_mod.linkSystemLibrary("m", .{});
        },
        .macos => {
            root_mod.addCSourceFiles(.{
                .root = b.path("src"),
                .files = &posix_sources,
                .flags = &c_flags,
            });
            root_mod.linkFramework("OpenGL", .{});
            root_mod.linkSystemLibrary("pthread", .{});
            root_mod.linkSystemLibrary("m", .{});
        },
        else => {
            std.debug.print("classicq: unsupported target OS '{s}'\n", .{@tagName(os_tag)});
            std.process.exit(1);
        },
    }

    b.installArtifact(exe);

    const install_to_assets = b.addUpdateSourceFiles();
    switch (os_tag) {
        .macos => {
            const bundle = "assets/classicq-macos-arm64.app";
            install_to_assets.addCopyFileToSource(
                exe.getEmittedBin(),
                bundle ++ "/Contents/MacOS/classicq-macos-arm64",
            );
            install_to_assets.addCopyFileToSource(
                b.path("assets/macos/Info.plist"),
                bundle ++ "/Contents/Info.plist",
            );
            install_to_assets.addCopyFileToSource(
                b.path("assets/icons/classicq.icns"),
                bundle ++ "/Contents/Resources/classicq.icns",
            );
        },
        else => {
            const bin_name: []const u8 = switch (os_tag) {
                .windows => "assets/classicq-windows-amd64.exe",
                .linux => "assets/classicq-linux-amd64",
                else => "assets/classicq",
            };
            install_to_assets.addCopyFileToSource(exe.getEmittedBin(), bin_name);
        },
    }
    b.getInstallStep().dependOn(&install_to_assets.step);

    const run_cmd = b.addRunArtifact(exe);
    run_cmd.step.dependOn(b.getInstallStep());
    if (b.args) |args| run_cmd.addArgs(args);
    const run_step = b.step("run", "Run classicQ");
    run_step.dependOn(&run_cmd.step);
}

const common_sources = [_][]const u8{
    "cd_null.c",
    "cl_cam.c",
    "cl_capture.c",
    "cl_cmd.c",
    "cl_demo.c",
    "cl_ents.c",
    "cl_fchecks.c",
    "cl_fragmsgs.c",
    "cl_ignore.c",
    "cl_input.c",
    "cl_logging.c",
    "cl_main.c",
    "cl_parse.c",
    "cl_pred.c",
    "cl_sbar.c",
    "cl_screen.c",
    "cl_tent.c",
    "cl_view.c",
    "cmd.c",
    "com_msg.c",
    "common.c",
    "config_manager.c",
    "console.c",
    "crc.c",
    "cvar.c",
    "filesystem.c",
    "fmod.c",
    "gl_draw.c",
    "gl_framebuffer.c",
    "gl_mesh.c",
    "gl_model.c",
    "gl_ngraph.c",
    "gl_part.c",
    "gl_post_process.c",
    "gl_refrag.c",
    "gl_rlight.c",
    "gl_rmain.c",
    "gl_rmisc.c",
    "gl_rpart.c",
    "gl_rsurf.c",
    "gl_shader.c",
    "gl_skinimp.c",
    "gl_state.c",
    "gl_texture.c",
    "gl_warp.c",
    "host.c",
    "huffman.c",
    "image.c",
    "keys.c",
    "linked_list.c",
    "lua.c",
    "match_tools.c",
    "mathlib.c",
    "md5.c",
    "mdfour.c",
    "menu.c",
    "modules.c",
    "mouse.c",
    "net.c",
    "net_chan.c",
    "netqw.c",
    "pmove.c",
    "pmovetst.c",
    "qstring.c",
    "r_draw.c",
    "r_part.c",
    "readablechars.c",
    "ruleset.c",
    "server_browser.c",
    "server_browser_qtv.c",
    "serverscanner.c",
    "skin.c",
    "sleep.c",
    "snd_main.c",
    "snd_mem.c",
    "snd_mix.c",
    "strlcat.c",
    "strlcpy.c",
    "sv_null.c",
    "tableprint.c",
    "teamplay.c",
    "text_input.c",
    "tokenize_string.c",
    "utils.c",
    "version.c",
    "vid.c",
    "vid_common_gl.c",
    "vid_mode_null.c",
    "wad.c",
    "zone.c",
};

const sdl_sources = [_][]const u8{
    "sys_sdl.c",
    "vid_sdl.c",
    "snd_sdl.c",
    "in_sdl.c",
};

const posix_sources = [_][]const u8{
    "net_posix.c",
    "sys_io_posix.c",
    "sys_lib_posix.c",
    "thread_posix.c",
};

const win_sources = [_][]const u8{
    "net_win32.c",
    "sys_io_win32.c",
    "sys_lib_null.c",
    "thread_win32.c",
};
