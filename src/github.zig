const std = @import("std");

const zdt = @import("zdt");

const log = std.log.scoped(.github);

pub const Releases = struct {
    parsed: std.json.Parsed([]Release),
    owned_memory: ?struct { mem: []const u8, allocator: std.mem.Allocator } = null,

    pub fn fromUrl(url: []const u8, allocator: std.mem.Allocator) !Releases {
        var client = std.http.Client{ .allocator = allocator };
        defer client.deinit();

        var result_body = std.io.Writer.Allocating.init(allocator);
        defer result_body.deinit();

        _ = try client.fetch(.{
            .location = .{ .url = url },
            .method = .GET,
            .response_writer = &result_body.writer,
        });

        const slice = try result_body.toOwnedSlice();
        var releases = Releases.fromSlice(slice, allocator) catch |err| {
            allocator.free(slice);
            return err;
        };
        releases.owned_memory = .{ .mem = slice, .allocator = allocator };
        return releases;
    }

    pub fn fromSlice(data: []const u8, allocator: std.mem.Allocator) !Releases {
        const parsed = try std.json.parseFromSlice([]Release, allocator, data, .{ .ignore_unknown_fields = true });
        return Releases{ .parsed = parsed };
    }

    pub fn deinit(self: Releases) void {
        self.parsed.deinit();
        if (self.owned_memory) |mem| {
            mem.allocator.free(mem.mem);
        }
    }

    pub fn latest(self: Releases) ?Release {
        if (self.parsed.value.len > 0) {
            return self.parsed.value[0];
        } else {
            return null;
        }
    }
};

const Release = struct {
    created_at: []const u8,
    assets: []const Asset,

    pub fn get_created_date(self: *const Release) ?zdt.Datetime {
        return zdt.Datetime.fromISO8601(self.created_at) catch {
            return null;
        };
    }

    pub fn get_asset(self: *const Release, name: []const u8) ?Asset {
        for (self.assets) |asset| {
            if (std.mem.eql(u8, asset.name, name)) {
                return asset;
            }
        }
        return null;
    }
};

const Asset = struct {
    name: []const u8,
    url: []const u8,
    content_type: []const u8,
    size: usize,
    digest: []const u8,
    browser_download_url: []const u8,

    pub fn get_sha256_digest(self: *const Asset) ?[]const u8 {
        if (std.mem.startsWith(u8, self.digest, "sha256:")) {
            return self.digest[7..];
        } else {
            return null;
        }
    }

    pub fn download(self: *const Asset, allocator: std.mem.Allocator, writer: *std.Io.Writer) !void {
        var client = std.http.Client{ .allocator = allocator };
        defer client.deinit();

        _ = try client.fetch(.{
            .location = .{ .url = self.browser_download_url },
            .method = .GET,
            .response_writer = writer,
        });
    }
};

pub fn fetch_latest_release(path: []const u8, release_url: []const u8, allocator: std.mem.Allocator) bool {
    const asset_name = std.fs.path.basename(path);
    if (std.fs.path.dirname(path)) |cache_dir| {
        std.fs.cwd().makePath(cache_dir) catch {};
    }

    const listfiles_release = Releases.fromUrl(release_url, allocator) catch |err| {
        log.warn("unable to fetch {s} releases - {}", .{ asset_name, err });
        std.fs.cwd().access(path, .{ .mode = .read_only }) catch {
            return false;
        };
        return true;
    };

    if (listfiles_release.latest()) |release| {
        if (release.get_asset(asset_name)) |asset| {
            if (asset.get_sha256_digest()) |digest| {
                if (validate_file_sha256(path, digest)) {
                    log.info("already have the latest {s}, so not fetching", .{asset_name});
                    return true;
                }
            }

            var client = std.http.Client{ .allocator = allocator };
            defer client.deinit();

            var file = std.fs.cwd().createFile(path, .{ .truncate = true }) catch |err| {
                log.err("unable to create {s} - {}", .{ path, err });
                return false;
            };
            defer file.close();

            var buffer: [8 * 1024]u8 = undefined;
            var writer = file.writer(&buffer);

            log.info("downloading {s}", .{asset_name});
            asset.download(allocator, &writer.interface) catch |err| {
                log.err("unable to download {s} - {}", .{ asset_name, err });
                return false;
            };
            writer.interface.flush() catch |err| {
                log.err("failed to flush {s} - {}", .{ path, err });
            };
        }
    }

    std.fs.cwd().access(path, .{ .mode = .read_only }) catch |err| {
        std.debug.panic("{s} doesn't exist - {}", .{ path, err });
    };
    return true;
}

fn validate_file_sha256(path: []const u8, hash_hex: []const u8) bool {
    var hash_buffer: [std.crypto.hash.sha2.Sha256.digest_length]u8 = undefined;
    const expected_hash = std.fmt.hexToBytes(&hash_buffer, hash_hex) catch {
        return false;
    };

    if (std.fs.cwd().openFile(path, .{ .mode = .read_only })) |existing_file| {
        defer existing_file.close();
        var hasher = std.crypto.hash.sha2.Sha256.init(.{});

        var buffer: [8 * 1024]u8 = undefined;
        var read_size = buffer.len;
        while (read_size >= buffer.len) {
            if (existing_file.readAll(&buffer)) |size| {
                read_size = size;
                hasher.update(buffer[0..read_size]);
            } else |_| {
                return false;
            }
        }
        const calculated_hash = hasher.finalResult();
        return std.mem.eql(u8, &calculated_hash, expected_hash);
    } else |_| {
        return false;
    }
}

const expect = std.testing.expect;

test "parse releases" {
    const allocator = std.heap.page_allocator;

    const releases = try Releases.fromSlice(example_releases, allocator);
    defer releases.deinit();

    try expect(std.mem.eql(u8, releases.parsed.value[0].created_at, "2026-02-01T21:08:04Z"));
}

const example_releases =
    \\[
    \\  {
    \\    "url": "https://api.github.com/repos/wowdev/wow-listfile/releases/281979888",
    \\    "assets_url": "https://api.github.com/repos/wowdev/wow-listfile/releases/281979888/assets",
    \\    "upload_url": "https://uploads.github.com/repos/wowdev/wow-listfile/releases/281979888/assets{?name,label}",
    \\    "html_url": "https://github.com/wowdev/wow-listfile/releases/tag/202602012108",
    \\    "id": 281979888,
    \\    "author": {
    \\      "login": "github-actions[bot]",
    \\      "id": 41898282,
    \\      "node_id": "MDM6Qm90NDE4OTgyODI=",
    \\      "avatar_url": "https://avatars.githubusercontent.com/in/15368?v=4",
    \\      "gravatar_id": "",
    \\      "url": "https://api.github.com/users/github-actions%5Bbot%5D",
    \\      "html_url": "https://github.com/apps/github-actions",
    \\      "followers_url": "https://api.github.com/users/github-actions%5Bbot%5D/followers",
    \\      "following_url": "https://api.github.com/users/github-actions%5Bbot%5D/following{/other_user}",
    \\      "gists_url": "https://api.github.com/users/github-actions%5Bbot%5D/gists{/gist_id}",
    \\      "starred_url": "https://api.github.com/users/github-actions%5Bbot%5D/starred{/owner}{/repo}",
    \\      "subscriptions_url": "https://api.github.com/users/github-actions%5Bbot%5D/subscriptions",
    \\      "organizations_url": "https://api.github.com/users/github-actions%5Bbot%5D/orgs",
    \\      "repos_url": "https://api.github.com/users/github-actions%5Bbot%5D/repos",
    \\      "events_url": "https://api.github.com/users/github-actions%5Bbot%5D/events{/privacy}",
    \\      "received_events_url": "https://api.github.com/users/github-actions%5Bbot%5D/received_events",
    \\      "type": "Bot",
    \\      "user_view_type": "public",
    \\      "site_admin": false
    \\    },
    \\    "node_id": "RE_kwDOAs2Gos4Qzqvw",
    \\    "tag_name": "202602012108",
    \\    "target_commitish": "master",
    \\    "name": "Full listfile v202602012108",
    \\    "draft": false,
    \\    "immutable": false,
    \\    "prerelease": false,
    \\    "created_at": "2026-02-01T21:08:04Z",
    \\    "updated_at": "2026-02-01T21:08:57Z",
    \\    "published_at": "2026-02-01T21:08:53Z",
    \\    "assets": [
    \\      {
    \\        "url": "https://api.github.com/repos/wowdev/wow-listfile/releases/assets/349209950",
    \\        "id": 349209950,
    \\        "node_id": "RA_kwDOAs2Gos4U0IVe",
    \\        "name": "community-listfile-withcapitals.csv",
    \\        "label": "",
    \\        "uploader": {
    \\          "login": "github-actions[bot]",
    \\          "id": 41898282,
    \\          "node_id": "MDM6Qm90NDE4OTgyODI=",
    \\          "avatar_url": "https://avatars.githubusercontent.com/in/15368?v=4",
    \\          "gravatar_id": "",
    \\          "url": "https://api.github.com/users/github-actions%5Bbot%5D",
    \\          "html_url": "https://github.com/apps/github-actions",
    \\          "followers_url": "https://api.github.com/users/github-actions%5Bbot%5D/followers",
    \\          "following_url": "https://api.github.com/users/github-actions%5Bbot%5D/following{/other_user}",
    \\          "gists_url": "https://api.github.com/users/github-actions%5Bbot%5D/gists{/gist_id}",
    \\          "starred_url": "https://api.github.com/users/github-actions%5Bbot%5D/starred{/owner}{/repo}",
    \\          "subscriptions_url": "https://api.github.com/users/github-actions%5Bbot%5D/subscriptions",
    \\          "organizations_url": "https://api.github.com/users/github-actions%5Bbot%5D/orgs",
    \\          "repos_url": "https://api.github.com/users/github-actions%5Bbot%5D/repos",
    \\          "events_url": "https://api.github.com/users/github-actions%5Bbot%5D/events{/privacy}",
    \\          "received_events_url": "https://api.github.com/users/github-actions%5Bbot%5D/received_events",
    \\          "type": "Bot",
    \\          "user_view_type": "public",
    \\          "site_admin": false
    \\        },
    \\        "content_type": "text/csv",
    \\        "state": "uploaded",
    \\        "size": 141489634,
    \\        "digest": "sha256:9b7279598535dd318828cbc73115f808fad49681c66f59d698efb3be3922989b",
    \\        "download_count": 0,
    \\        "created_at": "2026-02-01T21:08:54Z",
    \\        "updated_at": "2026-02-01T21:08:56Z",
    \\        "browser_download_url": "https://github.com/wowdev/wow-listfile/releases/download/202602012108/community-listfile-withcapitals.csv"
    \\      },
    \\      {
    \\        "url": "https://api.github.com/repos/wowdev/wow-listfile/releases/assets/349209953",
    \\        "id": 349209953,
    \\        "node_id": "RA_kwDOAs2Gos4U0IVh",
    \\        "name": "community-listfile.csv",
    \\        "label": "",
    \\        "uploader": {
    \\          "login": "github-actions[bot]",
    \\          "id": 41898282,
    \\          "node_id": "MDM6Qm90NDE4OTgyODI=",
    \\          "avatar_url": "https://avatars.githubusercontent.com/in/15368?v=4",
    \\          "gravatar_id": "",
    \\          "url": "https://api.github.com/users/github-actions%5Bbot%5D",
    \\          "html_url": "https://github.com/apps/github-actions",
    \\          "followers_url": "https://api.github.com/users/github-actions%5Bbot%5D/followers",
    \\          "following_url": "https://api.github.com/users/github-actions%5Bbot%5D/following{/other_user}",
    \\          "gists_url": "https://api.github.com/users/github-actions%5Bbot%5D/gists{/gist_id}",
    \\          "starred_url": "https://api.github.com/users/github-actions%5Bbot%5D/starred{/owner}{/repo}",
    \\          "subscriptions_url": "https://api.github.com/users/github-actions%5Bbot%5D/subscriptions",
    \\          "organizations_url": "https://api.github.com/users/github-actions%5Bbot%5D/orgs",
    \\          "repos_url": "https://api.github.com/users/github-actions%5Bbot%5D/repos",
    \\          "events_url": "https://api.github.com/users/github-actions%5Bbot%5D/events{/privacy}",
    \\          "received_events_url": "https://api.github.com/users/github-actions%5Bbot%5D/received_events",
    \\          "type": "Bot",
    \\          "user_view_type": "public",
    \\          "site_admin": false
    \\        },
    \\        "content_type": "text/csv",
    \\        "state": "uploaded",
    \\        "size": 141489634,
    \\        "digest": "sha256:1757cba906427982006f491fc9052a1b8318680ba5e42d3dc776dd0f94c53df9",
    \\        "download_count": 33,
    \\        "created_at": "2026-02-01T21:08:54Z",
    \\        "updated_at": "2026-02-01T21:08:57Z",
    \\        "browser_download_url": "https://github.com/wowdev/wow-listfile/releases/download/202602012108/community-listfile.csv"
    \\      },
    \\      {
    \\        "url": "https://api.github.com/repos/wowdev/wow-listfile/releases/assets/349209951",
    \\        "id": 349209951,
    \\        "node_id": "RA_kwDOAs2Gos4U0IVf",
    \\        "name": "lookup.csv",
    \\        "label": "",
    \\        "uploader": {
    \\          "login": "github-actions[bot]",
    \\          "id": 41898282,
    \\          "node_id": "MDM6Qm90NDE4OTgyODI=",
    \\          "avatar_url": "https://avatars.githubusercontent.com/in/15368?v=4",
    \\          "gravatar_id": "",
    \\          "url": "https://api.github.com/users/github-actions%5Bbot%5D",
    \\          "html_url": "https://github.com/apps/github-actions",
    \\          "followers_url": "https://api.github.com/users/github-actions%5Bbot%5D/followers",
    \\          "following_url": "https://api.github.com/users/github-actions%5Bbot%5D/following{/other_user}",
    \\          "gists_url": "https://api.github.com/users/github-actions%5Bbot%5D/gists{/gist_id}",
    \\          "starred_url": "https://api.github.com/users/github-actions%5Bbot%5D/starred{/owner}{/repo}",
    \\          "subscriptions_url": "https://api.github.com/users/github-actions%5Bbot%5D/subscriptions",
    \\          "organizations_url": "https://api.github.com/users/github-actions%5Bbot%5D/orgs",
    \\          "repos_url": "https://api.github.com/users/github-actions%5Bbot%5D/repos",
    \\          "events_url": "https://api.github.com/users/github-actions%5Bbot%5D/events{/privacy}",
    \\          "received_events_url": "https://api.github.com/users/github-actions%5Bbot%5D/received_events",
    \\          "type": "Bot",
    \\          "user_view_type": "public",
    \\          "site_admin": false
    \\        },
    \\        "content_type": "text/csv",
    \\        "state": "uploaded",
    \\        "size": 27665930,
    \\        "digest": "sha256:0945f2b370fd19ea26e4397b011ba87b45ef35eb317c1babfc70dabab0ed7405",
    \\        "download_count": 1,
    \\        "created_at": "2026-02-01T21:08:54Z",
    \\        "updated_at": "2026-02-01T21:08:55Z",
    \\        "browser_download_url": "https://github.com/wowdev/wow-listfile/releases/download/202602012108/lookup.csv"
    \\      },
    \\      {
    \\        "url": "https://api.github.com/repos/wowdev/wow-listfile/releases/assets/349209954",
    \\        "id": 349209954,
    \\        "node_id": "RA_kwDOAs2Gos4U0IVi",
    \\        "name": "verified-listfile-withcapitals.csv",
    \\        "label": "",
    \\        "uploader": {
    \\          "login": "github-actions[bot]",
    \\          "id": 41898282,
    \\          "node_id": "MDM6Qm90NDE4OTgyODI=",
    \\          "avatar_url": "https://avatars.githubusercontent.com/in/15368?v=4",
    \\          "gravatar_id": "",
    \\          "url": "https://api.github.com/users/github-actions%5Bbot%5D",
    \\          "html_url": "https://github.com/apps/github-actions",
    \\          "followers_url": "https://api.github.com/users/github-actions%5Bbot%5D/followers",
    \\          "following_url": "https://api.github.com/users/github-actions%5Bbot%5D/following{/other_user}",
    \\          "gists_url": "https://api.github.com/users/github-actions%5Bbot%5D/gists{/gist_id}",
    \\          "starred_url": "https://api.github.com/users/github-actions%5Bbot%5D/starred{/owner}{/repo}",
    \\          "subscriptions_url": "https://api.github.com/users/github-actions%5Bbot%5D/subscriptions",
    \\          "organizations_url": "https://api.github.com/users/github-actions%5Bbot%5D/orgs",
    \\          "repos_url": "https://api.github.com/users/github-actions%5Bbot%5D/repos",
    \\          "events_url": "https://api.github.com/users/github-actions%5Bbot%5D/events{/privacy}",
    \\          "received_events_url": "https://api.github.com/users/github-actions%5Bbot%5D/received_events",
    \\          "type": "Bot",
    \\          "user_view_type": "public",
    \\          "site_admin": false
    \\        },
    \\        "content_type": "text/csv",
    \\        "state": "uploaded",
    \\        "size": 72280179,
    \\        "digest": "sha256:72d155810b57ff75c5d94f160e131b413f6bf12f4883959770ee29d43d1b77ae",
    \\        "download_count": 2,
    \\        "created_at": "2026-02-01T21:08:54Z",
    \\        "updated_at": "2026-02-01T21:08:56Z",
    \\        "browser_download_url": "https://github.com/wowdev/wow-listfile/releases/download/202602012108/verified-listfile-withcapitals.csv"
    \\      },
    \\      {
    \\        "url": "https://api.github.com/repos/wowdev/wow-listfile/releases/assets/349209946",
    \\        "id": 349209946,
    \\        "node_id": "RA_kwDOAs2Gos4U0IVa",
    \\        "name": "verified-listfile.csv",
    \\        "label": "",
    \\        "uploader": {
    \\          "login": "github-actions[bot]",
    \\          "id": 41898282,
    \\          "node_id": "MDM6Qm90NDE4OTgyODI=",
    \\          "avatar_url": "https://avatars.githubusercontent.com/in/15368?v=4",
    \\          "gravatar_id": "",
    \\          "url": "https://api.github.com/users/github-actions%5Bbot%5D",
    \\          "html_url": "https://github.com/apps/github-actions",
    \\          "followers_url": "https://api.github.com/users/github-actions%5Bbot%5D/followers",
    \\          "following_url": "https://api.github.com/users/github-actions%5Bbot%5D/following{/other_user}",
    \\          "gists_url": "https://api.github.com/users/github-actions%5Bbot%5D/gists{/gist_id}",
    \\          "starred_url": "https://api.github.com/users/github-actions%5Bbot%5D/starred{/owner}{/repo}",
    \\          "subscriptions_url": "https://api.github.com/users/github-actions%5Bbot%5D/subscriptions",
    \\          "organizations_url": "https://api.github.com/users/github-actions%5Bbot%5D/orgs",
    \\          "repos_url": "https://api.github.com/users/github-actions%5Bbot%5D/repos",
    \\          "events_url": "https://api.github.com/users/github-actions%5Bbot%5D/events{/privacy}",
    \\          "received_events_url": "https://api.github.com/users/github-actions%5Bbot%5D/received_events",
    \\          "type": "Bot",
    \\          "user_view_type": "public",
    \\          "site_admin": false
    \\        },
    \\        "content_type": "text/csv",
    \\        "state": "uploaded",
    \\        "size": 72280179,
    \\        "digest": "sha256:67dde066c246e0a70cbe36c7028dd5bf6efb297e4ca16d72f5b34fc4a92c780b",
    \\        "download_count": 1,
    \\        "created_at": "2026-02-01T21:08:54Z",
    \\        "updated_at": "2026-02-01T21:08:55Z",
    \\        "browser_download_url": "https://github.com/wowdev/wow-listfile/releases/download/202602012108/verified-listfile.csv"
    \\      }
    \\    ],
    \\    "tarball_url": "https://api.github.com/repos/wowdev/wow-listfile/tarball/202602012108",
    \\    "zipball_url": "https://api.github.com/repos/wowdev/wow-listfile/zipball/202602012108",
    \\    "body": ""
    \\  }
    \\]
;
