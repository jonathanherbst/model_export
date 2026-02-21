const std = @import("std");

// Although this function looks imperative, it does not perform the build
// directly and instead it mutates the build graph (`b`) that will be then
// executed by an external runner. The functions in `std.Build` implement a DSL
// for defining build steps and express dependencies between them, allowing the
// build runner to parallelize the build automatically (and the cache system to
// know when a step doesn't need to be re-run).
pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const clap = b.dependency("clap", .{
        .target = target,
        .optimize = optimize,
    });

    const casclib = b.dependency("casclib", .{
        .target = target,
        .optimize = optimize,
    });

    const zdt = b.dependency("zdt", .{
        .target = target,
        .optimize = optimize,
    });

    const exe = b.addExecutable(.{
        .name = "model_export",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });
    exe.linkLibrary(casclib.artifact("casc"));
    exe.root_module.addImport("zdt", zdt.module("zdt"));
    b.installArtifact(exe);

    // db2util executable to parse db2 files
    const db2util = b.addExecutable(.{
        .name = "db2util",
        .use_llvm = true,
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/db2util.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });
    db2util.root_module.addImport("clap", clap.module("clap"));
    const install_db2util = b.addInstallArtifact(db2util, .{});

    // setup a step for the db2util so we can run it
    const run_db2util = b.step("db2util", "Run db2util");
    const cmd_db2util = b.addRunArtifact(db2util);
    run_db2util.dependOn(&cmd_db2util.step);
    cmd_db2util.step.dependOn(&install_db2util.step);
    if (b.args) |args| {
        cmd_db2util.addArgs(args);
    }

    // cascutil executable to interact with casc files
    const cascutil = b.addExecutable(.{
        .name = "cascutil",
        .use_llvm = true,
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/cascutil.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });
    cascutil.root_module.addImport("clap", clap.module("clap"));
    const install_cascutil = b.addInstallArtifact(cascutil, .{});

    // setup a step for the cascutil so we can run it
    const run_cascutil = b.step("cascutil", "Run cascutil");
    const cmd_cascutil = b.addRunArtifact(cascutil);
    run_cascutil.dependOn(&cmd_cascutil.step);
    cmd_cascutil.step.dependOn(&install_cascutil.step);
    if (b.args) |args| {
        cmd_cascutil.addArgs(args);
    }

    // tests
    // const mod_tests = b.addTest(.{
    //     .root_module = mod,
    // });
    // const run_mod_tests = b.addRunArtifact(mod_tests);
    // const exe_tests = b.addTest(.{
    //     .root_module = exe.root_module,
    // });
    // const run_exe_tests = b.addRunArtifact(exe_tests);
    // const test_step = b.step("test", "Run tests");
    // test_step.dependOn(&run_mod_tests.step);
    // test_step.dependOn(&run_exe_tests.step);
}
