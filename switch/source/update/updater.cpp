#include "update/updater.hpp"
#include "api/client.hpp"
#include "update/parsing.hpp"
#include "update/picosha2.h"
#include <algorithm>
#include <array>
#include <cctype>
#include <cerrno>
#include <chrono>
#include <cstdio>
#include <cstring>
#include <filesystem>
#include <limits>
#include <minizip/unzip.h>
#include <nlohmann/json.hpp>
#include <set>
#include <sys/stat.h>
#include <unistd.h>

namespace bird {
namespace {
using Json = nlohmann::json;
namespace fs = std::filesystem;
constexpr int64_t limit = 64LL * 1024 * 1024;
constexpr const char *nro_entry = "switch/birdroms/birdroms.nro";
constexpr const char *manifest_entry = "switch/birdroms/version.json";
constexpr const char *seed_entry = "switch/birdroms/server.json";
[[noreturn]] void fail(const std::string &message) {
  throw std::runtime_error(message);
}
using update_detail::digest_text;
using update_detail::version_parts;
bool exists(const std::string &p) {
  struct stat s {};
  if (lstat(p.c_str(), &s) == 0)
    return true;
  if (errno != ENOENT)
    fail("Cannot inspect update files");
  return false;
}
void regular(const std::string &p, bool required = true) {
  struct stat s {};
  if (lstat(p.c_str(), &s)) {
    if (!required && errno == ENOENT)
      return;
    fail("Cannot inspect update file");
  }
  if (!S_ISREG(s.st_mode))
    fail("Update path must be a regular file (no symlinks)");
}
void safe_path(const std::string &p) {
#ifdef __SWITCH__
  if (p.rfind("sdmc:/", 0) != 0)
    fail("Updater requires the actual sdmc executable path");
  size_t start = 6;
  std::string current = "sdmc:/";
#else
  if (p.empty() || p[0] != '/')
    fail("Updater requires an absolute executable path");
  size_t start = 1;
  std::string current = "/";
#endif
  if (p.find('\\') != std::string::npos || p.find('\0') != std::string::npos)
    fail("Invalid executable path");
  while (start < p.size()) {
    auto end = p.find('/', start);
    bool last = end == std::string::npos;
    auto component = p.substr(start, last ? p.size() - start : end - start);
    if (component.empty() || component == "." || component == ".." ||
        component.find(':') != std::string::npos)
      fail("Executable path contains unsafe components");
    current += component;
    if (!last) {
      struct stat s {};
      if (lstat(current.c_str(), &s) || !S_ISDIR(s.st_mode))
        fail("Executable parent must be a real directory");
      current += '/';
      start = end + 1;
    } else
      break;
  }
  if (p.size() < 4 || p.substr(p.size() - 4) != ".nro")
    fail("Updater requires an .nro executable path");
}
void remove_regular(const std::string &p) {
  if (exists(p)) {
    regular(p);
    if (std::remove(p.c_str()))
      fail("Cannot remove stale update file");
  }
}
struct File {
  FILE *p = nullptr;
  explicit File(const std::string &path, const char *mode) {
    p = fopen(path.c_str(), mode);
    if (!p)
      fail("Cannot open update file; check SD space and permissions");
  }
  ~File() {
    if (p)
      fclose(p);
  }
  void sync_close() {
    if (fflush(p) || fsync(fileno(p)))
      fail("Cannot flush update to SD");
    if (fclose(p)) {
      p = nullptr;
      fail("Cannot close update file");
    }
    p = nullptr;
  }
};
uint32_t u32(const unsigned char *p) {
  return uint32_t(p[0]) | (uint32_t(p[1]) << 8) | (uint32_t(p[2]) << 16) |
         (uint32_t(p[3]) << 24);
}
void validate_nro(const std::string &path, int64_t expected = 0) {
  regular(path);
  auto size = fs::file_size(path);
  if (size < 0x80 || size > uint64_t(limit) ||
      (expected && size != uint64_t(expected)))
    fail("Invalid NRO size");
  File f(path, "rb");
  std::array<unsigned char, 128> h{};
  if (fread(h.data(), 1, h.size(), f.p) != h.size() ||
      std::memcmp(h.data() + 0x10, "NRO0", 4))
    fail("Update is not a Nintendo Switch NRO");
  const uint64_t image = u32(h.data() + 0x18);
  if (image < 0x80 || image > size)
    fail("Invalid NRO image bounds");
  uint64_t previous = 0;
  for (size_t pos : {size_t(0x20), size_t(0x28), size_t(0x30)}) {
    uint64_t offset = u32(h.data() + pos), length = u32(h.data() + pos + 4);
    if (!length || offset < previous || offset + length > image)
      fail("Invalid NRO segment layout");
    previous = offset + length;
  }
  if (size > image) {
    std::array<unsigned char, 56> a{};
    if (size - image < a.size() ||
        fseek(f.p, static_cast<long>(image), SEEK_SET) ||
        fread(a.data(), 1, a.size(), f.p) != a.size() ||
        std::memcmp(a.data(), "ASET", 4))
      fail("Invalid NRO asset layout");
    auto u64 = [](const unsigned char *p) {
      return uint64_t(u32(p)) | (uint64_t(u32(p + 4)) << 32);
    };
    for (size_t pos : {size_t(8), size_t(24), size_t(40)}) {
      auto offset = u64(a.data() + pos), length = u64(a.data() + pos + 8);
      if (length && (offset < a.size() || offset > size - image ||
                     length > size - image - offset))
        fail("Invalid NRO asset bounds");
    }
  }
}
std::string hash_file(const std::string &path,
                      std::atomic_bool *cancel = nullptr) {
  File f(path, "rb");
  picosha2::hash256_one_by_one hash;
  std::array<unsigned char, 65536> b{};
  while (true) {
    if (cancel && cancel->load())
      fail("Update cancelled");
    auto n = fread(b.data(), 1, b.size(), f.p);
    if (n)
      hash.process(b.begin(), b.begin() + n);
    if (n < b.size()) {
      if (ferror(f.p))
        fail("Cannot read update file");
      break;
    }
  }
  hash.finish();
  return picosha2::get_hash_hex_string(hash);
}
void write_json(const std::string &path, const Json &j) {
  regular(path, false);
  File f(path, "wb");
  auto data = j.dump();
  if (fwrite(data.data(), 1, data.size(), f.p) != data.size())
    fail("Cannot write update manifest");
  f.sync_close();
}
Json parse_archive_json(const std::string &raw) {
  std::vector<std::set<std::string>> objects;
  try {
    return Json::parse(raw, [&](int, Json::parse_event_t event, Json &value) {
      if (event == Json::parse_event_t::object_start)
        objects.emplace_back();
      else if (event == Json::parse_event_t::object_end)
        objects.pop_back();
      else if (event == Json::parse_event_t::key &&
               !objects.back().insert(value.get<std::string>()).second)
        fail("Duplicate JSON field in Switch update archive");
      return true;
    });
  } catch (const Json::exception &) {
    fail("Malformed Switch update archive JSON");
  }
}
UpdateInfo manifest_info(const Json &j) {
  if (!j.is_object() || j.size() != 4 || !j.contains("platform") ||
      !j.contains("version") || !j.contains("nro_sha256") ||
      !j.contains("nro_size"))
    fail("Unexpected fields in Switch update manifest");
  if (j.at("platform").get<std::string>() != "switch")
    fail("Update manifest targets another platform");
  UpdateInfo i;
  i.available = true;
  i.version = j.at("version").get<std::string>();
  version_parts(i.version);
  i.nro_sha256 = digest_text(j, "nro_sha256");
  i.nro_size = j.at("nro_size").get<int64_t>();
  if (i.nro_size < 128 || i.nro_size > limit)
    fail("Invalid manifest NRO size");
  return i;
}
Json manifest_json(const UpdateInfo &i) {
  return Json{{"platform", "switch"},
              {"version", i.version},
              {"nro_sha256", i.nro_sha256},
              {"nro_size", i.nro_size}};
}
void verify_file(const std::string &p, const UpdateInfo &i,
                 std::atomic_bool *cancel = nullptr) {
  validate_nro(p, i.nro_size);
  if (hash_file(p, cancel) != i.nro_sha256)
    fail("NRO SHA256 verification failed");
}
std::string entry_text(unzFile zip, size_t cap, std::atomic_bool &cancel) {
  std::string out;
  std::array<char, 8192> b{};
  while (true) {
    if (cancel.load())
      fail("Update cancelled");
    int n = unzReadCurrentFile(zip, b.data(), b.size());
    if (n < 0)
      fail("Cannot decompress update metadata");
    if (!n)
      break;
    if (size_t(n) > cap - out.size())
      fail("Update metadata is too large");
    out.append(b.data(), n);
  }
  return out;
}
} // namespace
bool update_needed(const std::string &installed, const std::string &offered) {
  return version_parts(offered) > version_parts(installed);
}
std::string update_stage_path(const std::string &p) { return p + ".update"; }
std::string update_backup_path(const std::string &p) {
  if (p.size() < 4)
    return p + ".previous.nro";
  return p.substr(0, p.size() - 4) + ".previous.nro";
}
std::string update_executable_path(const std::string &argv0) {
  try {
    safe_path(argv0);
    regular(argv0);
    return argv0;
  } catch (...) {
    return {};
  }
}
void stage_update_archive(const std::string &archive, const std::string &target,
                          const UpdateInfo &expected,
                          std::atomic_bool &cancel) {
  safe_path(target);
  regular(target);
  regular(archive);
  if (fs::file_size(archive) > uint64_t(limit))
    fail("Update ZIP exceeds 64 MiB limit");
  if (!expected.available)
    fail("No update package available");
  version_parts(expected.version);
  auto temp = update_stage_path(target) + ".tmp",
       stage = update_stage_path(target), sidecar = stage + ".json",
       side_tmp = sidecar + ".tmp";
  regular(stage, false);
  regular(sidecar, false);
  remove_regular(temp);
  remove_regular(side_tmp);
  unzFile zip = unzOpen64(archive.c_str());
  if (!zip)
    fail("Cannot open update ZIP");
  try {
    std::set<std::string> seen;
    Json manifest;
    bool have_nro = false, have_manifest = false;
    size_t entries = 0;
    int result = unzGoToFirstFile(zip);
    while (result == UNZ_OK) {
      if (cancel.load())
        fail("Update cancelled");
      if (++entries > 8)
        fail("Unexpected files in Switch update ZIP");
      unz_file_info64 info{};
      if (unzGetCurrentFileInfo64(zip, &info, nullptr, 0, nullptr, 0, nullptr,
                                  0) != UNZ_OK ||
          info.size_filename > 1024)
        fail("Malformed ZIP entry");
      std::vector<char> namebuf(info.size_filename + 1);
      if (unzGetCurrentFileInfo64(zip, &info, namebuf.data(), namebuf.size(),
                                  nullptr, 0, nullptr, 0) != UNZ_OK)
        fail("Cannot read ZIP entry");
      std::string name(namebuf.data(), info.size_filename);
      if (name.find('\0') != std::string::npos || !seen.insert(name).second)
        fail("Duplicate or malformed ZIP entry");
      auto mode = (info.external_fa >> 16) & 0170000;
      if (mode == 0120000 || (mode && mode != 0100000 && mode != 0040000))
        fail("ZIP links and special files are forbidden");
      bool directory = name == "switch/" || name == "switch/birdroms/";
      if (name != nro_entry && name != manifest_entry && name != seed_entry &&
          !directory)
        fail("Unexpected path in Switch update ZIP");
      if (directory) {
        if (mode == 0100000)
          fail("ZIP directory has regular file type");
        if (info.uncompressed_size)
          fail("Nonempty ZIP directory");
        result = unzGoToNextFile(zip);
        continue;
      }
      if (mode == 0040000 || info.flag & 1)
        fail("Invalid or encrypted ZIP entry");
      if (info.uncompressed_size > uint64_t(name == nro_entry ? limit : 65536))
        fail("Update entry exceeds size limit");
      if (unzOpenCurrentFile(zip) != UNZ_OK)
        fail("Cannot decompress ZIP entry");
      try {
        if (name == nro_entry) {
          if (info.uncompressed_size != uint64_t(expected.nro_size))
            fail("NRO does not match advertised size");
          File out(temp, "wb");
          std::array<unsigned char, 65536> b{};
          uint64_t written = 0;
          while (true) {
            if (cancel.load())
              fail("Update cancelled");
            int n = unzReadCurrentFile(zip, b.data(), b.size());
            if (n < 0)
              fail("Cannot decompress NRO");
            if (!n)
              break;
            if (uint64_t(n) > uint64_t(limit) - written)
              fail("NRO exceeds size limit");
            if (fwrite(b.data(), 1, n, out.p) != size_t(n))
              fail("Cannot write update; check free SD space");
            written += n;
          }
          if (written != uint64_t(expected.nro_size))
            fail("Truncated NRO");
          out.sync_close();
          have_nro = true;
        } else {
          auto raw = entry_text(zip, 65536, cancel);
          auto j = parse_archive_json(raw);
          if (name == manifest_entry) {
            manifest = j;
            have_manifest = true;
          } else {
            if (!j.is_object() || j.size() != 1 || !j.contains("server_url") ||
                !j["server_url"].is_string())
              fail("Invalid server seed (credentials forbidden)");
          }
        }
      } catch (...) {
        unzCloseCurrentFile(zip);
        throw;
      }
      if (unzCloseCurrentFile(zip) != UNZ_OK)
        fail("Update ZIP CRC verification failed");
      result = unzGoToNextFile(zip);
    }
    if (result != UNZ_END_OF_LIST_OF_FILE || !have_nro || !have_manifest)
      fail("Incomplete Switch update ZIP");
    auto actual = manifest_info(manifest);
    if (actual.version != expected.version ||
        actual.nro_size != expected.nro_size ||
        actual.nro_sha256 != expected.nro_sha256)
      fail("ZIP manifest differs from authenticated update metadata");
    verify_file(temp, expected, &cancel);
    write_json(side_tmp, manifest_json(expected));
    if (cancel.load())
      fail("Update cancelled");
    remove_regular(stage);
    remove_regular(sidecar);
    if (std::rename(temp.c_str(), stage.c_str()))
      fail("Cannot stage verified update");
    if (std::rename(side_tmp.c_str(), sidecar.c_str())) {
      remove_regular(stage);
      fail("Cannot stage verified update manifest");
    }
    unzClose(zip);
  } catch (...) {
    unzClose(zip);
    std::remove(temp.c_str());
    std::remove(side_tmp.c_str());
    throw;
  }
}
void apply_staged_update(const std::string &target) {
  safe_path(target);
  auto stage = update_stage_path(target), sidecar = stage + ".json",
       backup = update_backup_path(target);
  if (!exists(stage))
    return;
  regular(stage);
  regular(sidecar);
  regular(target, false);
  regular(backup, false);
  UpdateInfo info;
  {
    File in(sidecar, "rb");
    std::array<char, 65537> buf{};
    auto n = fread(buf.data(), 1, buf.size(), in.p);
    if (n == buf.size() || ferror(in.p))
      fail("Invalid staged update manifest");
    info = manifest_info(parse_archive_json(std::string(buf.data(), n)));
  } // Switch cannot delete the sidecar while its read handle is open.

  verify_file(stage, info);
  if (!exists(target)) {
    if (!exists(backup))
      fail("Original executable missing; install update manually");
    if (std::rename(backup.c_str(), target.c_str()))
      fail("Cannot recover previous executable");
  }
  validate_nro(target);
  remove_regular(backup);
  if (std::rename(target.c_str(), backup.c_str()))
    fail("Cannot preserve previous executable");
  if (std::rename(stage.c_str(), target.c_str())) {
    if (std::rename(backup.c_str(), target.c_str()))
      fail("Update failed; launch the .previous.nro backup");
    fail("Update failed; previous executable restored");
  }
  std::remove(sidecar.c_str()); // Keep the launchable .previous.nro for manual
                                // rollback/power-loss recovery.
}
Updater::Updater(Config c, std::string executable, std::string current)
    : config_(std::move(c)), executable_(update_executable_path(executable)),
      version_(std::move(current)) {}
Updater::~Updater() {
  cancel();
  if (worker_.joinable())
    worker_.join();
}
void Updater::cancel() { cancel_ = true; }
UpdateState Updater::snapshot() const {
  std::lock_guard<std::mutex> lock(mutex_);
  return state_;
}
void Updater::check() {
  {
    std::lock_guard<std::mutex> lock(mutex_);
    if (state_.running)
      return;
    state_.running = true;
    state_.checked = false;
    state_.cancelled = false;
    state_.ready = false;
    state_.message = "Checking this server for Switch updates...";
  }
  if (worker_.joinable())
    worker_.join();
  cancel_ = false;
  worker_ = std::thread([this] {
    try {
      auto info = Client(config_).handheld_info(&cancel_);
      bool offered = info.available && update_needed(version_, info.version);
      std::lock_guard<std::mutex> lock(mutex_);
      state_.info = info;
      state_.checked = true;
      state_.available = offered && !executable_.empty();
      state_.cancelled = cancel_.load();
      state_.message = cancel_ ? "Update check cancelled"
                       : executable_.empty()
                           ? "Launch from a real SD .nro path to enable updates"
                       : offered ? "Switch update available: " + info.version
                                 : "Already running the latest Switch build";
      state_.running = false;
    } catch (const std::exception &e) {
      std::lock_guard<std::mutex> lock(mutex_);
      state_.running = false;
      state_.checked = true;
      state_.available = false;
      state_.cancelled = cancel_.load();
      state_.message = cancel_ ? "Update check cancelled" : e.what();
    }
  });
}
void Updater::start() {
  UpdateInfo info;
  {
    std::lock_guard<std::mutex> lock(mutex_);
    if (state_.running || !state_.available || state_.ready)
      return;
    info = state_.info;
    state_.running = true;
    state_.cancelled = false;
    state_.written = 0;
    state_.total = info.size;
    state_.message = "Downloading Switch update...";
  }
  if (worker_.joinable())
    worker_.join();
  cancel_ = false;
  worker_ = std::thread([this, info] {
    auto zip = update_stage_path(executable_) + ".zip.part";
    try {
      safe_path(executable_);
      regular(executable_);
      remove_regular(zip);
      File out(zip, "wb");
      auto began = std::chrono::steady_clock::now();
      Client(config_).download_handheld(
          out.p, cancel_, [this, began](int64_t n, int64_t total) {
            std::lock_guard<std::mutex> lock(mutex_);
            state_.written = n;
            if (total > 0)
              state_.total = total;
            double elapsed = std::chrono::duration<double>(
                                 std::chrono::steady_clock::now() - began)
                                 .count();
            state_.speed = elapsed > 0 ? n / elapsed : 0;
          });
      out.sync_close();
      {
        std::lock_guard<std::mutex> lock(mutex_);
        state_.message = "Verifying and staging Switch update...";
      }
      stage_update_archive(zip, executable_, info, cancel_);
      std::remove(zip.c_str());
      std::lock_guard<std::mutex> lock(mutex_);
      state_.running = false;
      state_.ready = true;
      state_.message =
          "Update verified. Exit birdROMs to install; then relaunch.";
    } catch (const std::exception &e) {
      std::remove(zip.c_str());
      std::lock_guard<std::mutex> lock(mutex_);
      state_.running = false;
      // A failed automatic attempt stays stopped until an explicit metadata
      // check.
      state_.available = false;
      state_.cancelled = cancel_.load();
      state_.message = cancel_ ? "Update cancelled" : e.what();
    }
  });
}
} // namespace bird
