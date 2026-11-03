#pragma once
#include "update/updater.hpp"
#include <cctype>
#include <nlohmann/json.hpp>
#include <stdexcept>
namespace bird {
namespace update_detail {
using Json = nlohmann::json;
constexpr int64_t limit = 64LL * 1024 * 1024;
[[noreturn]] inline void fail(const std::string &message) {
  throw std::runtime_error(message);
}
inline std::vector<uint32_t> version_parts(const std::string &value) {
  std::string s = value;
  if (!s.empty() && s.front() == 'v')
    s.erase(0, 1);
  if (s.empty())
    fail("Missing application version");
  std::vector<uint32_t> parts;
  uint64_t n = 0;
  bool digits = false;
  for (unsigned char c : s) {
    if (c == '.') {
      if (!digits)
        fail("Invalid application version");
      parts.push_back(n);
      n = 0;
      digits = false;
    } else if (c >= '0' && c <= '9') {
      digits = true;
      n = n * 10 + c - '0';
      if (n > UINT32_MAX)
        fail("Application version component too large");
    } else
      fail("Application version must contain numeric date components");
  }
  if (!digits)
    fail("Invalid application version");
  parts.push_back(n);
  if (parts.size() != 3 && parts.size() != 4)
    fail("Invalid date application version");
  if (parts.size() == 3)
    parts.push_back(0);
  if (parts[0] < 2020 || parts[1] < 1 || parts[1] > 12 || parts[2] < 1)
    fail("Invalid date application version");
  const uint32_t year = parts[0], month = parts[1], day = parts[2];
  const bool leap = year % 4 == 0 && (year % 100 != 0 || year % 400 == 0);
  const uint32_t days[] = {31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31};
  if (day > days[month - 1] + (month == 2 && leap ? 1 : 0))
    fail("Invalid calendar date in application version");
  return parts;
}
inline std::string digest_text(const Json &j, const char *key) {
  auto s = j.at(key).get<std::string>();
  if (s.size() != 64)
    fail("Invalid update checksum");
  for (char &c : s) {
    if (!std::isxdigit(static_cast<unsigned char>(c)))
      fail("Invalid update checksum");
    c = std::tolower(static_cast<unsigned char>(c));
  }
  return s;
}
} // namespace update_detail
inline UpdateInfo parse_update_info(const std::string &raw) {
  using namespace update_detail;
  try {
    auto j = Json::parse(raw);
    UpdateInfo i;
    i.available = j.at("available").get<bool>();
    if (!i.available)
      return i;
    if (j.at("platform").get<std::string>() != "switch")
      fail("Update metadata targets another platform");
    i.version = j.at("version").get<std::string>();
    version_parts(i.version);
    i.filename = j.at("filename").get<std::string>();
    i.sha256 = digest_text(j, "sha256");
    i.nro_sha256 = digest_text(j, "nro_sha256");
    i.size = j.at("size").get<int64_t>();
    i.nro_size = j.at("nro_size").get<int64_t>();
    if (i.filename.empty() ||
        i.filename.find_first_of("/\\") != std::string::npos || i.size <= 0 ||
        i.size > limit || i.nro_size < 128 || i.nro_size > limit)
      fail("Invalid Switch update metadata");
    return i;
  } catch (const Json::exception &) {
    fail("Malformed Switch update metadata");
  }
}
} // namespace bird
