#include "config/storage.hpp"
#include "downloads/manager.hpp"
#include <cassert>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <thread>
#include <unistd.h>
namespace fs = std::filesystem;
using namespace bird;
static int calls = 0;
static bool fail = false, unauthorized = false;
static std::atomic_bool await_cancel{false}, entered{false};
namespace bird {
void Client::download(int, FILE *f, std::atomic_bool &cancel,
                      const std::function<void(int64_t, int64_t)> &progress) {
  ++calls;
  if (unauthorized)
    throw HttpError(401, "Device revoked");
  if (await_cancel) {
    entered = true;
    while (!cancel)
      std::this_thread::sleep_for(std::chrono::milliseconds(1));
    throw std::runtime_error("cancelled");
  }
  if (cancel)
    throw std::runtime_error("cancelled");
  fwrite("abc", 1, 3, f);
  progress(3, 3);
  if (fail)
    throw std::runtime_error("mock transport failure");
}
} // namespace bird
static void wait(Downloads &d) {
  while (d.snapshot().running)
    std::this_thread::sleep_for(std::chrono::milliseconds(1));
}
template <class F> void rejects(F f) {
  bool rejected = false;
  try {
    f();
  } catch (const std::exception &) {
    rejected = true;
  }
  assert(rejected);
}
int main() {
  auto root = fs::temp_directory_path() /
              ("birdroms-storage-test-" + std::to_string(getpid()));
  fs::remove_all(root);
  fs::create_directories(root);
  Config c;
  c.server_url = "https://example.test";
  c.device_token = "test-only";
  c.rom_base = (root / "roms").string();
  c.platform_dirs["Custom"] = "custom";
  auto config = (root / "config.json").string();
  save_config(config, c);
  auto loaded = load_config(config);
  assert(loaded.paired());
  assert(loaded.server_url == c.server_url &&
         loaded.device_token == c.device_token &&
         loaded.rom_base == c.rom_base &&
         loaded.platform_dirs == c.platform_dirs);
  assert(!fs::exists(config + ".tmp"));
  assert(!load_config((root / "absent").string()).paired());
  {
    std::ofstream f(config);
    f << "{broken";
  }
  rejects([&] { load_config(config); });
  assert(platform_directory(c, "NES") == "nes");
  assert(platform_directory(c, "Nintendo - Nintendo 64") == "n64");
  assert(platform_directory(c, "Sony PlayStation") == "psx");
  assert(platform_directory(c, "Custom") == "custom");
  assert(platform_directory(c, "../../evil") == "_UNSORTED");
  Rom r;
  r.id = 42;
  r.title = "Zelda";
  r.platform = "NES";
  r.extension = ".nes";
  r.filename = "Zelda (USA).nes";
  r.size_bytes = 3;
  assert(filename(r) == r.filename);
  r.filename = "../../evil.nes";
  assert(filename(r).find('/') == std::string::npos);
  r.filename = "..";
  assert(filename(r) == "Zelda.nes");
  r.filename = "";
  r.title = "Zelda.NES";
  assert(filename(r) == "Zelda.nes");
  r.title = "Zelda";
  r.filename = "Zelda (USA).nes";
  Config bad = c;
  bad.platform_dirs["NES"] = "../escape";
  rejects([&] { destination(bad, r); });
  bad = c;
  bad.rom_base = (root / "roms" / ".." / "escape").string();
  rejects([&] { destination(bad, r); });
  fs::create_directories(root / "outside");
  fs::create_directory_symlink(root / "outside", root / "linked");
  bad = c;
  bad.rom_base = (root / "linked").string();
  rejects([&] { destination(bad, r); });
  assert(!existing(c, r));
  {
    Downloads d;
    d.start(c, {r});
    wait(d);
    auto s = d.snapshot();
    assert(s.done == 1 && s.failed == 0 && s.finished);
    assert(calls == 1);
    assert(fs::file_size(root / "roms" / "nes" / r.filename) == 3);
    assert(!fs::exists(root / "roms" / "nes" / (r.filename + ".part")));
  }
  assert(existing(c, r));
  {
    Downloads d;
    d.start(c, {r});
    wait(d);
    assert(d.snapshot().skipped == 1 && calls == 1);
  }
  fs::rename(root / "roms" / "nes" / r.filename,
             root / "roms" / "nes" / "Zelda [!].nes");
  assert(existing(c, r));
  fs::remove_all(root / "roms");
  fail = true;
  {
    Downloads d;
    d.start(c, {r});
    wait(d);
    assert(d.snapshot().failed == 1 && !d.snapshot().last_error.empty());
    assert(!fs::exists(root / "roms" / "nes" / r.filename));
    assert(fs::file_size(root / "roms" / "nes" / (r.filename + ".part")) == 3);
  }
  fail = false;
  {
    Downloads d;
    d.start(c, {r});
    wait(d);
    assert(d.snapshot().done == 1);
    assert(fs::file_size(root / "roms" / "nes" / r.filename) == 3);
  }
  // Different data occupying the original name is preserved, with an id suffix
  // for our ROM.
  fs::remove_all(root / "roms");
  fs::create_directories(root / "roms" / "nes");
  {
    std::ofstream f(root / "roms" / "nes" / r.filename);
    f << "other-data";
  }
  {
    Downloads d;
    d.start(c, {r});
    wait(d);
    assert(d.snapshot().done == 1);
    assert(fs::file_size(root / "roms" / "nes" / r.filename) == 10);
    assert(fs::file_size(root / "roms" / "nes" / "Zelda (USA)-42.nes") == 3);
  }

  // A successful transport carrying fewer bytes than metadata stays partial.
  fs::remove_all(root / "roms");
  r.size_bytes = 6;
  {
    Downloads d;
    d.start(c, {r});
    wait(d);
    assert(d.snapshot().failed == 1 && !d.snapshot().last_error.empty());
    assert(!fs::exists(root / "roms" / "nes" / r.filename));
    assert(fs::file_size(root / "roms" / "nes" / (r.filename + ".part")) == 3);
  }
  // Cancellation stops the active request and does not begin later ROMs.
  fs::remove_all(root / "roms");
  r.size_bytes = 3;
  await_cancel = true;
  entered = false;
  int before = calls;
  {
    Downloads d;
    d.start(c, {r, r});
    while (!entered)
      std::this_thread::sleep_for(std::chrono::milliseconds(1));
    d.cancel();
    wait(d);
    auto state = d.snapshot();
    assert(state.finished && !state.running && state.failed == 0 &&
           state.done == 0);
    assert(calls == before + 1);
    assert(!fs::exists(root / "roms" / "nes" / r.filename));
    assert(fs::exists(root / "roms" / "nes" / (r.filename + ".part")));
  }
  await_cancel = false;
  fs::remove_all(root / "roms");
  unauthorized = true;
  {
    Downloads d;
    const int before = calls;
    d.start(c, {r, r});
    wait(d);
    assert(d.snapshot().unauthorized && d.snapshot().failed == 1);
    assert(calls == before + 1);
  }
  unauthorized = false;
  fs::remove_all(root);
  std::cout << "Storage tests passed: configuration, mappings, filename/path "
               "safety, symlinks, existing detection, queue failures, partial "
               "restart, collisions\n";
}
