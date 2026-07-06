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

    const sdl_dep = b.dependency("sdl", .{
        .target = target,
        .optimize = optimize,
    });
    root_mod.linkLibrary(sdl_dep.artifact("SDL3"));

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

    // shaders_gen.h built from the committed blobs in src/shaders/compiled
    const wf = b.addWriteFiles();
    _ = wf.add("shaders_gen.h", makeShaderHeader(b) catch @panic("src/shaders/compiled unreadable"));
    root_mod.addIncludePath(wf.getDirectory());

    addShadercrossStep(b);

    const c_flags = [_][]const u8{
        "-std=c23",
        "-D_GNU_SOURCE",
        "-DCLIENTONLY",
        "-DNETQW",
        "-DGLQUAKE",
        "-DUSE_PNG=1",
        "-DUSE_JPEG=0",
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
    "gl_mesh.c",
    "gl_model.c",
    "gl_refrag.c",
    "gpu_alias.c",
    "gpu_draw2d.c",
    "gpu_part.c",
    "gpu_rmain.c",
    "gpu_stub.c",
    "gpu_texture.c",
    "gpu_vid.c",
    "gpu_world.c",
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

fn shaderNameLessThan(_: void, a: []const u8, c: []const u8) bool {
    return std.mem.lessThan(u8, a, c);
}

// C header with one byte array per blob, symbol = filename with dots as underscores
fn makeShaderHeader(b: *std.Build) ![]const u8 {
    const alloc = b.allocator;
    const io = b.graph.io;

    var dir = try b.build_root.handle.openDir(io, "src/shaders/compiled", .{ .iterate = true });
    defer dir.close(io);

    var names: std.ArrayList([]const u8) = .empty;
    var it = dir.iterate();
    while (try it.next(io)) |e| {
        if (e.kind == .file)
            try names.append(alloc, b.dupe(e.name));
    }
    std.mem.sort([]const u8, names.items, {}, shaderNameLessThan);

    var out: std.ArrayList(u8) = .empty;
    try out.appendSlice(alloc, "// generated by build.zig from src/shaders/compiled, do not edit\n\n");
    for (names.items) |name| {
        const data = try dir.readFileAlloc(io, name, alloc, .unlimited);
        const sym = b.dupe(name);
        for (sym) |*ch| {
            if (ch.* == '.') ch.* = '_';
        }
        try out.appendSlice(alloc, b.fmt("static const unsigned char {s}[] = {{\n", .{sym}));
        var buf: [8]u8 = undefined;
        for (data, 0..) |byte, i| {
            const s = std.fmt.bufPrint(&buf, "{d},", .{byte}) catch unreachable;
            try out.appendSlice(alloc, s);
            if (i % 16 == 15)
                try out.append(alloc, '\n');
        }
        try out.appendSlice(alloc, "\n};\n");
    }
    return out.items;
}

// zig build shaders: HLSL -> SPIRV/DXIL/MSL via shadercross (PATH or SHADERCROSS env)
fn addShadercrossStep(b: *std.Build) void {
    const step = b.step("shaders", "Recompile HLSL shaders into src/shaders/compiled (needs shadercross)");
    const exe = b.graph.environ_map.get("SHADERCROSS") orelse "shadercross";

    const io = b.graph.io;
    var dir = b.build_root.handle.openDir(io, "src/shaders", .{ .iterate = true }) catch @panic("src/shaders unreadable");
    defer dir.close(io);

    var it = dir.iterate();
    while (it.next(io) catch @panic("src/shaders unreadable")) |e| {
        if (e.kind != .file or !std.mem.endsWith(u8, e.name, ".hlsl"))
            continue;
        const base = e.name[0 .. e.name.len - 5];
        for ([_][]const u8{ "spv", "dxil", "msl" }) |fmt| {
            const run = b.addSystemCommand(&.{ exe, b.dupe(e.name), "-o", b.fmt("compiled/{s}.{s}", .{ base, fmt }) });
            run.setCwd(b.path("src/shaders"));
            step.dependOn(&run.step);
        }
    }
}
