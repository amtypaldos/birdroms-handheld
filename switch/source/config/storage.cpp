#include "config/storage.hpp"
#include "nlohmann/json.hpp"
#include <algorithm>
#include <cerrno>
#include <cstdio>
#include <cstring>
#include <fcntl.h>
#include <filesystem>
#include <fstream>
#include <stdexcept>
#include <unistd.h>
namespace bird {
namespace fs = std::filesystem;
namespace {
std::string lower(std::string s) {
  for (char &c : s)
    if ((unsigned char)c < 128)
      c = std::tolower((unsigned char)c);
  return s;
}
std::string trim(std::string s) {
  while (!s.empty() && (s.back() == '.' || s.back() == ' '))
    s.pop_back();
  size_t p = s.find_first_not_of(' ');
  return p == std::string::npos ? "" : s.substr(p);
}
std::string sanitize(const std::string &s) {
  std::string out;
  for (unsigned char c : s) {
    if (c < 32 || c == 127)
      continue;
    out += std::string("/\\:*?\"<>|").find(c) != std::string::npos ? '_' : c;
  }
  return trim(out);
}
std::string truncate(std::string s, size_t n) {
  if (s.size() <= n)
    return s;
  size_t end = n;
  while (end > 0 && ((unsigned char)s[end] & 0xc0) == 0x80)
    --end;
  return s.substr(0, end);
}
void segment(const std::string &s) {
  if (s.empty() || s == "." || s == ".." || s != sanitize(s) || s.size() > 240)
    throw std::runtime_error("Invalid platform directory mapping");
}
void safe_path(const fs::path &p) {
  for (const auto &item : p) {
    if (item == "..")
      throw std::runtime_error("Parent path traversal is forbidden");
  }
  fs::path current;
  for (const auto &item : p) {
    current /= item;
    std::error_code ec;
    auto st = fs::symlink_status(current, ec);
    if (!ec && fs::is_symlink(st))
      throw std::runtime_error("Symbolic links are forbidden in ROM paths");
  }
}
void base_valid(const std::string &base) {
#ifdef __SWITCH__
  if (base.rfind("sdmc:/", 0) != 0)
    throw std::runtime_error("ROM base directory must begin sdmc:/");
#else
  if (base.rfind("sdmc:/", 0) != 0 && !fs::path(base).is_absolute())
    throw std::runtime_error("ROM base directory must be absolute");
#endif
  safe_path(base);
}
std::string stem(std::string s) { return fs::path(s).stem().string(); }
std::string normalized(const std::string &s) {
  std::string out;
  int depth = 0;
  bool space = false;
  for (unsigned char c : lower(s)) {
    if (c == '(' || c == '[') {
      ++depth;
      continue;
    }
    if (c == ')' || c == ']') {
      if (depth)
        --depth;
      continue;
    }
    if (depth)
      continue;
    if (std::isalnum(c) || c >= 128) {
      if (space && !out.empty())
        out += ' ';
      out += c;
      space = false;
    } else
      space = true;
  }
  return out;
}
std::string id_name(const Rom &r) {
  auto n = filename(r);
  auto ext = fs::path(n).extension().string();
  auto base = n.substr(0, n.size() - ext.size());
  auto suffix = "-" + std::to_string(r.id);
  return trim(truncate(base, 250 - ext.size() - suffix.size())) + suffix + ext;
}
bool present(const fs::path &p) {
  std::error_code ec;
  return fs::exists(fs::symlink_status(p, ec));
}
} // namespace
Config load_config(const std::string &path) {
  Config c;
  std::ifstream f(path);
  if (!f) {
    if (!fs::exists(path))
      return c;
    throw std::runtime_error("Cannot read configuration");
  }
  try {
    nlohmann::json j;
    f >> j;
    c.version = j.value("version", 1);
    if (c.version != 1)
      throw std::runtime_error("Unsupported configuration version");
    c.server_url = j.value("server_url", "");
    c.device_token = j.value("device_token", "");
    c.rom_base = j.value("rom_base", c.rom_base);
    if (j.contains("platform_dirs"))
      c.platform_dirs =
          j.at("platform_dirs").get<std::map<std::string, std::string>>();
    base_valid(c.rom_base);
    for (auto &kv : c.platform_dirs)
      segment(kv.second);
    return c;
  } catch (const nlohmann::json::exception &) {
    throw std::runtime_error("Malformed configuration");
  }
}
void save_config(const std::string &path, const Config &c) {
  base_valid(c.rom_base);
  for (auto &kv : c.platform_dirs)
    segment(kv.second);
  fs::path p(path);
  safe_path(p);
  if (!p.parent_path().empty())
    fs::create_directories(p.parent_path());
  nlohmann::json j = {{"version", 1},
                      {"server_url", c.server_url},
                      {"device_token", c.device_token},
                      {"rom_base", c.rom_base},
                      {"platform_dirs", c.platform_dirs}};
  std::string data = j.dump(2) + "\n";
  std::string temp = path + ".tmp";
  safe_path(temp);
  int fd = open(temp.c_str(), O_WRONLY | O_CREAT | O_TRUNC, 0600);
  if (fd < 0)
    throw std::runtime_error("Cannot create configuration temporary file");
  size_t offset = 0;
  bool ok = true;
  while (offset < data.size()) {
    auto n = write(fd, data.data() + offset, data.size() - offset);
    if (n <= 0) {
      ok = false;
      break;
    }
    offset += (size_t)n;
  }
  if (ok && fsync(fd) != 0)
    ok = false;
  if (close(fd) != 0)
    ok = false;
  if (!ok) {
    std::remove(temp.c_str());
    throw std::runtime_error("Cannot write configuration");
  }
  if (std::rename(temp.c_str(), path.c_str()) != 0) {
    std::remove(temp.c_str());
    throw std::runtime_error("Cannot replace configuration");
  }
}
std::string filename(const Rom &r) {
  std::string full = sanitize(r.filename);
  std::string ext, base;
  if (!full.empty()) {
    ext = truncate(fs::path(full).extension().string(), 32);
    base = trim(stem(full));
  }
  if (base.empty()) {
    base = sanitize(r.title);
    ext = truncate(sanitize(r.extension), 32);
    if (!ext.empty() && base.size() > ext.size() &&
        lower(base.substr(base.size() - ext.size())) == lower(ext))
      base.resize(base.size() - ext.size());
  }
  base = trim(truncate(base, 250 - ext.size()));
  if (base.empty())
    base = "rom-" + std::to_string(r.id);
  return base + ext;
}
std::string platform_directory(const Config &c, const std::string &p) {
  auto custom = c.platform_dirs.find(p);
  if (custom != c.platform_dirs.end()) {
    segment(custom->second);
    return custom->second;
  }
  static const std::map<std::string, std::string> defaults = {
#include "config/platform_map.inc"
  };
  auto it = defaults.find(lower(p));
  return it == defaults.end() ? "_UNSORTED" : it->second;
}
std::string destination(const Config &c, const Rom &r) {
  base_valid(c.rom_base);
  fs::path dir = fs::path(c.rom_base) / platform_directory(c, r.platform);
  safe_path(dir);
  fs::path out = dir / filename(r);
  safe_path(out);
  if (present(out) && !existing(c, r))
    out = dir / id_name(r);
  safe_path(out);
  return out.string();
}
bool existing(const Config &c, const Rom &r) {
  base_valid(c.rom_base);
  fs::path dir = fs::path(c.rom_base) / platform_directory(c, r.platform);
  safe_path(dir);
  std::error_code ec;
  if (!fs::is_directory(dir, ec))
    return false;
  auto clean = lower(filename(r)), alt = lower(id_name(r));
  for (const auto &e : fs::directory_iterator(dir)) {
    if (!e.is_regular_file() || e.is_symlink())
      continue;
    auto n = e.path().filename().string();
    if (n.empty() || n[0] == '.')
      continue;
    auto ln = lower(n);
    bool exact = ln == clean || ln == alt;
    if (exact && (r.size_bytes <= 0 || e.file_size() == (uint64_t)r.size_bytes))
      return true;
    if (r.size_bytes > 0 && e.file_size() == (uint64_t)r.size_bytes &&
        lower(e.path().extension().string()) == lower(r.extension)) {
      auto k = normalized(e.path().stem().string());
      if (!k.empty() && (k == normalized(stem(filename(r))) ||
                         k == normalized(stem(r.title))))
        return true;
    }
  }
  return false;
}
} // namespace bird
