#include "api/client.hpp"
#include "api/parsing.hpp"
#include <algorithm>
#include <cctype>
#include <cerrno>
#include <cstring>
#include <curl/curl.h>
#include <limits>
#include <memory>

#ifndef BIRDROMS_VERSION
#define BIRDROMS_VERSION "dev"
#endif

namespace bird {
namespace {
using Json = nlohmann::json;
using Handle = std::unique_ptr<CURL, decltype(&curl_easy_cleanup)>;
constexpr size_t max_json = 16 * 1024 * 1024, max_cover = 4 * 1024 * 1024;
std::string base(const Config &c) {
  auto s = c.server_url;
  while (!s.empty() && s.back() == '/')
    s.pop_back();
  return s;
}
std::string part(CURLU *u, CURLUPart p) {
  char *raw = nullptr;
  if (curl_url_get(u, p, &raw, CURLU_DEFAULT_PORT))
    return {};
  std::string s(raw);
  curl_free(raw);
  return s;
}
std::string origin(const std::string &value) {
  std::unique_ptr<CURLU, decltype(&curl_url_cleanup)> u(curl_url(),
                                                        curl_url_cleanup);
  if (!u || curl_url_set(u.get(), CURLUPART_URL, value.c_str(), 0))
    throw std::runtime_error("Invalid HTTP URL");
  auto scheme = part(u.get(), CURLUPART_SCHEME);
  auto host = part(u.get(), CURLUPART_HOST);
  if ((scheme != "https" && scheme != "http") || host.empty() ||
      !part(u.get(), CURLUPART_USER).empty() ||
      !part(u.get(), CURLUPART_PASSWORD).empty())
    throw std::runtime_error(
        "URL must use HTTP(S) without embedded credentials");
  std::transform(host.begin(), host.end(), host.begin(),
                 [](unsigned char c) { return std::tolower(c); });
  return scheme + "://" + host + ":" + part(u.get(), CURLUPART_PORT);
}
Handle setup(const std::string &url) {
  origin(url);
  Handle h(curl_easy_init(), curl_easy_cleanup);
  if (!h)
    throw std::runtime_error("Cannot initialize HTTP client");
  curl_easy_setopt(h.get(), CURLOPT_URL, url.c_str());
#if LIBCURL_VERSION_NUM >= 0x075500
  curl_easy_setopt(h.get(), CURLOPT_PROTOCOLS_STR, "http,https");
#else
  curl_easy_setopt(h.get(), CURLOPT_PROTOCOLS,
                   CURLPROTO_HTTP | CURLPROTO_HTTPS);
#endif
  curl_easy_setopt(h.get(), CURLOPT_FOLLOWLOCATION, 0L);
  curl_easy_setopt(h.get(), CURLOPT_CONNECTTIMEOUT, 15L);
  curl_easy_setopt(h.get(), CURLOPT_LOW_SPEED_LIMIT, 1L);
  curl_easy_setopt(h.get(), CURLOPT_LOW_SPEED_TIME, 30L);
  curl_easy_setopt(h.get(), CURLOPT_NOSIGNAL, 1L);
  curl_easy_setopt(h.get(), CURLOPT_SSL_VERIFYPEER, 1L);
  curl_easy_setopt(h.get(), CURLOPT_SSL_VERIFYHOST, 2L);
#ifdef __SWITCH__
  // libnx sslCreateContext uses the system CertStore automatically.
  // Do not assume the native service imports concatenated PEM bundles.
  curl_easy_setopt(h.get(), CURLOPT_CAINFO, static_cast<const char *>(nullptr));
  curl_easy_setopt(h.get(), CURLOPT_CAPATH, static_cast<const char *>(nullptr));
#endif
  curl_easy_setopt(h.get(), CURLOPT_USERAGENT, "birdROMs-Switch/" BIRDROMS_VERSION);
  return h;
}
struct Headers {
  curl_slist *p = nullptr;
  ~Headers() { curl_slist_free_all(p); }
  void add(const std::string &s) {
    auto next = curl_slist_append(p, s.c_str());
    if (!next)
      throw std::runtime_error("Cannot allocate HTTP headers");
    p = next;
  }
};
void auth(Headers &headers, const Config &c, bool authenticated) {
  headers.add("Accept: application/json");
  if (authenticated) {
    if (c.device_token.find_first_of("\r\n") != std::string::npos)
      throw std::runtime_error("Invalid device token");
    headers.add("Authorization: Device " + c.device_token);
  }
}
struct Buffer {
  std::vector<unsigned char> bytes;
  size_t limit;
  bool overflow = false, failed = false;
};
size_t buffered(char *data, size_t size, size_t n, void *ptr) noexcept {
  auto &b = *static_cast<Buffer *>(ptr);
  if (n && size > std::numeric_limits<size_t>::max() / n) {
    b.overflow = true;
    return 0;
  }
  size_t count = size * n;
  if (count > b.limit - b.bytes.size()) {
    b.overflow = true;
    return 0;
  }
  try {
    b.bytes.insert(b.bytes.end(), data, data + count);
    return count;
  } catch (...) {
    b.failed = true;
    return 0;
  }
}
struct Reply {
  long status;
  std::vector<unsigned char> body;
};
Reply request(const Config &c, const std::string &url, bool authenticated,
              bool post = false, size_t cap = max_json) {
  auto h = setup(url);
  Headers headers;
  auth(headers, c, authenticated);
  curl_easy_setopt(h.get(), CURLOPT_HTTPHEADER, headers.p);
  curl_easy_setopt(h.get(), CURLOPT_TIMEOUT, 45L);
  if (post) {
    curl_easy_setopt(h.get(), CURLOPT_POST, 1L);
    curl_easy_setopt(h.get(), CURLOPT_POSTFIELDS, "");
    curl_easy_setopt(h.get(), CURLOPT_POSTFIELDSIZE, 0L);
  }
  Buffer b{{}, cap};
  curl_easy_setopt(h.get(), CURLOPT_WRITEFUNCTION, buffered);
  curl_easy_setopt(h.get(), CURLOPT_WRITEDATA, &b);
  const auto result = curl_easy_perform(h.get());
  long status = 0;
  curl_easy_getinfo(h.get(), CURLINFO_RESPONSE_CODE, &status);
  if (b.overflow)
    throw std::runtime_error("Server response exceeds memory limit");
  if (b.failed)
    throw std::runtime_error("Not enough memory for server response");
  if (result != CURLE_OK)
    throw HttpError(0, std::string("Network request failed: ") +
                           curl_easy_strerror(result));
  return {status, std::move(b.bytes)};
}
void success(long s) {
  if (s < 200 || s >= 300)
    throw HttpError(static_cast<int>(s),
                    s == 401 ? "Device token revoked or expired; pair again"
                             : "Server returned HTTP " + std::to_string(s));
}
Json decode(const Reply &r) {
  try {
    return Json::parse(r.body.begin(), r.body.end());
  } catch (...) {
    throw std::runtime_error("Malformed JSON response");
  }
}
std::string escape(const std::string &s) {
  Handle h(curl_easy_init(), curl_easy_cleanup);
  if (!h)
    throw std::runtime_error("Cannot initialize HTTP client");
  char *p = curl_easy_escape(h.get(), s.c_str(), static_cast<int>(s.size()));
  if (!p)
    throw std::runtime_error("Cannot encode request");
  std::string out(p);
  curl_free(p);
  return out;
}
Json get(const Config &c, const std::string &p) {
  auto r = request(c, base(c) + p, true);
  success(r.status);
  return decode(r);
}
struct Stream {
  CURL *curl;
  FILE *file;
  std::atomic_bool *cancel;
  const std::function<void(int64_t, int64_t)> *progress;
  bool write_failed = false, callback_failed = false;
};
size_t streamed(char *data, size_t size, size_t n, void *p) noexcept {
  auto &s = *static_cast<Stream *>(p);
  if (s.cancel->load())
    return 0;
  long status = 0;
  curl_easy_getinfo(s.curl, CURLINFO_RESPONSE_CODE, &status);
  if (status != 200)
    return size * n; // Never put error documents or unsolicited partial data in
                     // a .part file.
  if (size && n > std::numeric_limits<size_t>::max() / size) {
    s.write_failed = true;
    return 0;
  }
  size_t count = size * n, written = fwrite(data, 1, count, s.file);
  if (written != count)
    s.write_failed = true;
  return written;
}
int progress(void *p, curl_off_t total, curl_off_t now, curl_off_t,
             curl_off_t) noexcept {
  auto &s = *static_cast<Stream *>(p);
  if (s.cancel->load())
    return 1;
  try {
    if (*s.progress)
      (*s.progress)(static_cast<int64_t>(now), static_cast<int64_t>(total));
  } catch (...) {
    s.callback_failed = true;
    return 1;
  }
  return 0;
}
} // namespace
std::vector<std::string> Client::platforms() {
  try {
    return get(config_, "/api/roms/platforms/").get<std::vector<std::string>>();
  } catch (const Json::exception &) {
    throw std::runtime_error("Malformed platform list");
  }
}
std::vector<Playlist> Client::playlists() {
  try {
    auto j = get(config_, "/api/playlists/");
    if (!j.is_array())
      throw std::runtime_error("Malformed playlist list");
    std::vector<Playlist> out;
    for (auto &v : j)
      out.push_back({v.at("id").get<int>(), parsing::text(v, "name"),
                     v.value("rom_count", 0)});
    return out;
  } catch (const Json::exception &) {
    throw std::runtime_error("Malformed playlist list");
  }
}
Page Client::roms(const std::string &platform, int page) {
  if (page < 1)
    throw std::runtime_error("Page must be positive");
  try {
    return parsing::page(
        get(config_,
            "/api/roms/?fields=device&page=" + std::to_string(page) +
                (platform.empty() ? "" : "&platform=" + escape(platform))));
  } catch (const Json::exception &) {
    throw std::runtime_error("Malformed ROM page");
  }
}
std::vector<Rom> Client::playlist(int id) {
  if (id <= 0)
    throw std::runtime_error("Invalid playlist identifier");
  try {
    return parsing::rom_list(
        get(config_, "/api/playlists/" + std::to_string(id) + "/").at("roms"));
  } catch (const Json::exception &) {
    throw std::runtime_error("Malformed playlist detail");
  }
}
PairSession Client::pair_start() {
  auto r = request(config_, base(config_) + "/api/devices/pair/start", false,
                   true, 65536);
  if (r.status != 201) {
    success(r.status);
    throw HttpError(static_cast<int>(r.status),
                    "Unexpected pairing start status");
  }
  try {
    return parsing::session(decode(r));
  } catch (const Json::exception &) {
    throw std::runtime_error("Malformed pairing response");
  }
}
PollResult Client::pair_poll(const std::string &code) {
  auto r = request(
      config_, base(config_) + "/api/devices/pair/poll?code=" + escape(code),
      false, false, 65536);
  if (r.status == 202 || r.status == 410 || r.status == 429)
    return parsing::poll(r.status);
  if (r.status != 200) {
    success(r.status);
    throw HttpError(static_cast<int>(r.status),
                    "Unexpected pairing poll status");
  }
  try {
    return parsing::poll(r.status, decode(r));
  } catch (const Json::exception &) {
    throw std::runtime_error("Malformed pairing approval response");
  }
}
std::vector<unsigned char> Client::cover(const std::string &url) {
  auto r = request(config_, url, origin(url) == origin(base(config_)), false,
                   max_cover);
  success(r.status);
  return std::move(r.body);
}
void Client::download(int id, FILE *file, std::atomic_bool &cancel,
                      const std::function<void(int64_t, int64_t)> &callback) {
  if (id <= 0 || !file)
    throw std::runtime_error("Invalid download destination");
  if (cancel.load())
    throw std::runtime_error("Download cancelled");
  auto h =
      setup(base(config_) + "/api/roms/" + std::to_string(id) + "/download/");
  Headers headers;
  auth(headers, config_, true);
  curl_easy_setopt(h.get(), CURLOPT_HTTPHEADER, headers.p);
  Stream s{h.get(), file, &cancel, &callback};
  curl_easy_setopt(h.get(), CURLOPT_WRITEFUNCTION, streamed);
  curl_easy_setopt(h.get(), CURLOPT_WRITEDATA, &s);
  curl_easy_setopt(h.get(), CURLOPT_NOPROGRESS, 0L);
  curl_easy_setopt(h.get(), CURLOPT_XFERINFOFUNCTION, progress);
  curl_easy_setopt(h.get(), CURLOPT_XFERINFODATA, &s);
  auto result = curl_easy_perform(h.get());
  long status = 0;
  curl_easy_getinfo(h.get(), CURLINFO_RESPONSE_CODE, &status);
  if (cancel.load())
    throw std::runtime_error("Download cancelled");
  if (s.write_failed || ferror(file))
    throw std::runtime_error("SD write failed (check free space and card)");
  if (s.callback_failed)
    throw std::runtime_error("Download progress callback failed");
  if (status)
    success(status);
  if (status >= 200 && status < 300 && status != 200)
    throw HttpError(static_cast<int>(status),
                    "Download expected a complete HTTP 200 response");
  if (result != CURLE_OK)
    throw HttpError(0, std::string("Download failed: ") +
                           curl_easy_strerror(result));
}
} // namespace bird
