#include "ui/app.hpp"
#include "api/client.hpp"
#include "config/folders.hpp"
#include "config/storage.hpp"
#include "downloads/manager.hpp"
#include "ui/image_bounds.hpp"
#include "ui/state.hpp"
#include <SDL.h>
#include <SDL_image.h>
#include <SDL_ttf.h>
#include <switch.h>
extern "C" {
#include "platform/qrcodegen.h"
}
#include <chrono>
#include <future>
#include <iomanip>
#include <memory>
#include <set>
#include <sstream>
namespace bird {
namespace {
using Clock = std::chrono::steady_clock;
const SDL_Color bg{11, 13, 23, 255}, neon{0, 240, 255, 255},
    pink{255, 45, 149, 255}, dim{138, 144, 168, 255}, panel{22, 26, 44, 255};
std::string bytes(int64_t n) {
  std::ostringstream s;
  s << std::fixed << std::setprecision(1);
  if (n > 1073741824)
    s << n / 1073741824.0 << " GB";
  else if (n > 1048576)
    s << n / 1048576.0 << " MB";
  else if (n > 1024)
    s << n / 1024.0 << " KB";
  else
    s << n << " B";
  return s.str();
}
struct Result {
  std::vector<std::string> platforms;
  std::vector<Playlist> playlists;
  Page page;
  PairSession session;
  PollResult poll{PollStatus::Pending, {}};
  std::string error;
  int status = 0;
};
enum class Job { None, Home, Open, More, All, PairStart, PairPoll };
enum class Screen { Setup, Pair, Home, Grid, Download, Directory };
struct FolderResult {
  FolderPage page;
  std::string error;
};
struct CoverResult {
  std::vector<unsigned char> data;
  int status = 0;
};
struct Cover {
  SDL_Texture *texture = nullptr;
  uint64_t stamp = 0;
};
class App {
  Config cfg;
  std::string cfgpath;
  SDL_Window *window = nullptr;
  SDL_Renderer *renderer = nullptr;
  TTF_Font *normal = nullptr, *small = nullptr, *big = nullptr;
  PadState pad{};
  Screen screen = Screen::Home, download_return = Screen::Home;
  Selection selection;
  Downloads downloads;
  bool quit = false;
  int focus = 0, top = 0, homefocus = 0, hometop = 0, page = 0, total = 0;
  bool next = false, selecting_all = false;
  std::string title, platform, message;
  std::vector<Rom> roms;
  std::vector<std::string> platforms;
  std::vector<Playlist> playlists;
  std::future<Result> future;
  Job job = Job::None;
  unsigned generation = 0, jobgeneration = 0;
  PairSession session;
  int pairfails = 0, restarts = 0;
  Clock::time_point nextpoll = Clock::now(), receipt = Clock::now();
  std::future<CoverResult> coverfuture;
  int coverid = 0;
  std::map<int, Cover> covers;
  std::set<int> coverfailed;
  uint64_t frame = 0, repeatAt = 0;
  u64 lastdirection = 0;
  std::string qrfor;
  std::vector<uint8_t> qr;
  bool qrvalid = false;
  static constexpr const char *sdroot = "sdmc:/";
  FolderPage folder_page;
  int folder_focus = 0, folder_top = 0;
  std::future<FolderResult> folder_future;
  std::string folder_request;
  bool folder_initial = false;
  unsigned folder_generation = 0, folder_job_generation = 0;
  bool folder_busy() const {
    return folder_future.valid() || !folder_request.empty();
  }
  void request_folder(const std::string &path, bool initial = false) {
    ++folder_generation;
    folder_request = path;
    folder_initial = initial;
    message.clear();
  }
  void collect_folders() {
    if (folder_future.valid() && folder_future.wait_for(std::chrono::seconds(
                                     0)) == std::future_status::ready) {
      auto result = folder_future.get();
      if (screen == Screen::Directory &&
          folder_job_generation == folder_generation) {
        if (!result.error.empty())
          message = result.error;
        else {
          folder_page = std::move(result.page);
          folder_focus = folder_top = 0;
        }
      }
    }
    if (screen == Screen::Directory && !folder_future.valid() &&
        !folder_request.empty()) {
      auto path = std::move(folder_request);
      folder_request.clear();
      bool initial = folder_initial;
      folder_job_generation = folder_generation;
      folder_future = std::async(std::launch::async, [path, initial]() {
        FolderResult result;
        try {
          result.page =
              list_folders(sdroot, initial ? folder_start(sdroot, path) : path);
        } catch (const std::exception &) {
          result.error =
              "Cannot read this SD directory. B goes back; X cancels.";
        }
        return result;
      });
    }
  }
  void close_folders() {
    ++folder_generation;
    folder_request.clear();
    screen = Screen::Home;
    message.clear();
  }
  void press_folders(u64 keys) {
    if (keys & HidNpadButton_X) {
      close_folders();
      return;
    }
    if (keys & HidNpadButton_B) {
      if (folder_busy() || folder_page.path.empty() ||
          folder_page.path == sdroot)
        close_folders();
      else
        request_folder(folder_parent(sdroot, folder_page.path));
      return;
    }
    if (folder_busy())
      return;
    const int count = folder_page.directories.size();
    if (keys & HidNpadButton_Up)
      folder_focus = move_focus(folder_focus, -1, count);
    if (keys & HidNpadButton_Down)
      folder_focus = move_focus(folder_focus, 1, count);
    if (keys & HidNpadButton_L)
      folder_focus = move_focus(folder_focus, -8, count);
    if (keys & HidNpadButton_R)
      folder_focus = move_focus(folder_focus, 8, count);
    if (folder_focus < folder_top)
      folder_top = folder_focus;
    if (folder_focus >= folder_top + 8)
      folder_top = folder_focus - 7;
    if (keys & HidNpadButton_A && count) {
      request_folder(folder_child(sdroot, folder_page.path,
                                  folder_page.directories[folder_focus]));
      return;
    }
    if (keys & HidNpadButton_Y && !folder_page.path.empty()) {
      auto updated = cfg;
      updated.rom_base = folder_page.path;
      try {
        save_paired_config(cfgpath, updated);
        cfg = std::move(updated);
        close_folders();
        message = "ROM directory saved: " + cfg.rom_base;
      } catch (const std::exception &e) {
        message = std::string("Could not save ROM directory: ") + e.what();
      }
    }
  }
  void draw_folders() {
    text("ROM DIRECTORY", 40, 20, neon, big);
    text("Saved: " + cfg.rom_base, 40, 80, dim, small, 1190);
    text(folder_page.path.empty() ? "Loading SD card..." : folder_page.path, 40,
         125, pink, normal, 1190);
    int end = std::min<int>(folder_page.directories.size(), folder_top + 8);
    for (int i = folder_top; i < end; ++i) {
      int y = 185 + (i - folder_top) * 43;
      if (i == folder_focus) {
        fill(panel, {35, y - 2, 1210, 40});
        fill(neon, {35, y - 2, 5, 40});
      }
      text(folder_page.directories[i] + "/", 60, y,
           i == folder_focus ? neon : dim, normal, 1160);
    }
    if (folder_page.directories.empty() && !folder_busy())
      text("No subdirectories. Y chooses this folder.", 60, 220);
    text("System folders (nes/, snes/, gba/, psx/...) are added beneath your "
         "choice.",
         40, 550, dim, small);
    text("Applies to future downloads. Existing ROMs stay in their current "
         "folders.",
         40, 580, dim, small);
    if (folder_busy())
      text("Reading SD card...", 40, 635, neon, small);
    text("A open    Y use this folder    B parent/back    X cancel    L/R page",
         40, 675, dim, small);
  }
  void fill(SDL_Color c, SDL_Rect r) {
    SDL_SetRenderDrawColor(renderer, c.r, c.g, c.b, 255);
    SDL_RenderFillRect(renderer, &r);
  }
  void outline(SDL_Color c, SDL_Rect r) {
    SDL_SetRenderDrawColor(renderer, c.r, c.g, c.b, 255);
    SDL_RenderDrawRect(renderer, &r);
  }
  void text(const std::string &s, int x, int y, SDL_Color c = dim,
            TTF_Font *f = nullptr, int maxw = 1180) {
    if (!f)
      f = normal;
    std::string clipped = s;
    int w = 0, h = 0;
    TTF_SizeUTF8(f, clipped.c_str(), &w, &h);
    while (w > maxw && !clipped.empty()) {
      size_t cut = clipped.size() - 1;
      while (cut > 0 &&
             (static_cast<unsigned char>(clipped[cut]) & 0xc0) == 0x80)
        --cut;
      clipped.resize(cut);
      TTF_SizeUTF8(f, clipped.c_str(), &w, &h);
    }
    if (clipped.empty())
      return;
    auto *srf = TTF_RenderUTF8_Blended(f, clipped.c_str(), c);
    if (!srf)
      return;
    auto *t = SDL_CreateTextureFromSurface(renderer, srf);
    SDL_Rect r{x, y, srf->w, srf->h};
    SDL_FreeSurface(srf);
    if (t) {
      SDL_RenderCopy(renderer, t, nullptr, &r);
      SDL_DestroyTexture(t);
    }
  }
  template <class F> void start(Job kind, F fn) {
    if (job != Job::None)
      return;
    job = kind;
    jobgeneration = generation;
    future = std::async(std::launch::async, [fn]() {
      try {
        return fn();
      } catch (const HttpError &e) {
        Result r;
        r.error = e.what();
        r.status = e.status;
        return r;
      } catch (const std::exception &e) {
        Result r;
        r.error = e.what();
        return r;
      }
    });
  }
  void home() {
    screen = Screen::Home;
    message.clear();
    start(Job::Home, [c = cfg]() {
      Client api(c);
      Result r;
      r.platforms = api.platforms();
      r.playlists = api.playlists();
      return r;
    });
  }
  void pair() {
    screen = Screen::Pair;
    session = {};
    message.clear();
    nextpoll = Clock::now();
    pairfails = 0;
    restarts = 0;
  }
  void request_pair() {
    if (job != Job::None || Clock::now() < nextpoll || !message.empty())
      return;
    if (session.code.empty())
      start(Job::PairStart, [c = cfg]() {
        Result r;
        r.session = Client(c).pair_start();
        return r;
      });
    else
      start(Job::PairPoll, [c = cfg, code = session.code]() {
        Result r;
        r.poll = Client(c).pair_poll(code);
        return r;
      });
  }
  void open() {
    if (homefocus < (int)playlists.size()) {
      auto p = playlists[homefocus];
      title = p.name;
      platform.clear();
      start(Job::Open, [c = cfg, id = p.id]() {
        Result r;
        r.page.results = Client(c).playlist(id);
        r.page.count = r.page.results.size();
        return r;
      });
    } else {
      platform = platforms[homefocus - playlists.size()];
      title = platform;
      start(Job::Open, [c = cfg, p = platform]() {
        Result r;
        r.page = Client(c).roms(p, 1);
        return r;
      });
    }
  }
  void more(bool all = false) {
    if (!next || job != Job::None)
      return;
    message.clear();
    start(all ? Job::All : Job::More, [c = cfg, p = platform, n = page + 1]() {
      Result r;
      r.page = Client(c).roms(p, n);
      return r;
    });
  }
  void collect() {
    if (job == Job::None ||
        future.wait_for(std::chrono::seconds(0)) != std::future_status::ready)
      return;
    auto r = future.get();
    Job kind = job;
    job = Job::None;
    if (jobgeneration != generation)
      return;
    if (!r.error.empty()) {
      selecting_all = false;
      if (r.status == 401 && cfg.paired()) {
        cfg.device_token.clear();
        selection.clear();
        pair();
        return;
      }
      if (kind == Job::PairStart || kind == Job::PairPoll) {
        if ((r.status == 0 || r.status >= 500) && ++pairfails < 3) {
          nextpoll = Clock::now() + std::chrono::seconds(2);
          return;
        }
      }
      selecting_all = false;
      message = r.error;
      return;
    }
    message.clear();
    if (kind == Job::Home) {
      platforms = std::move(r.platforms);
      playlists = std::move(r.playlists);
      homefocus = move_focus(homefocus, 0, platforms.size() + playlists.size());
    } else if (kind == Job::Open) {
      selecting_all = false;
      screen = Screen::Grid;
      roms = std::move(r.page.results);
      next = r.page.next;
      total = r.page.count;
      page = 1;
      focus = top = 0;
    } else if (kind == Job::More || kind == Job::All) {
      roms.insert(roms.end(), r.page.results.begin(), r.page.results.end());
      page++;
      next = r.page.next;
      total = r.page.count;
      if (kind == Job::All || selecting_all) {
        if (next)
          more(true);
        else {
          selecting_all = false;
          selection.select_all(roms);
        }
      }
    } else if (kind == Job::PairStart) {
      session = r.session;
      receipt = Clock::now();
      nextpoll = Clock::now();
      pairfails = 0;
    } else if (kind == Job::PairPoll) {
      pairfails = 0;
      if (r.poll.status == PollStatus::Approved) {
        cfg.device_token = r.poll.token;
        try {
          save_paired_config(cfgpath, cfg);
          home();
        } catch (const std::exception &e) {
          message = std::string("Token received; save failed: ") + e.what();
        }
        return;
      }
      if (r.poll.status == PollStatus::Expired) {
        if (restarts++ == 0) {
          session = {};
          nextpoll = Clock::now();
        } else
          message = "Pairing code expired. A retries; B exits.";
      } else
        nextpoll = Clock::now() +
                   std::chrono::seconds(
                       r.poll.status == PollStatus::RateLimited ? 2 : 1);
    }
  }
  void cover_update() {
    if (coverfuture.valid() && coverfuture.wait_for(std::chrono::seconds(0)) ==
                                   std::future_status::ready) {
      auto result = coverfuture.get();
      auto data = std::move(result.data);
      if (result.status == 401 && cfg.paired()) {
        generation++;
        cfg.device_token.clear();
        selection.clear();
        pair();
      }
      SDL_Texture *t = nullptr;
      if (cover_dimensions_safe(data)) {
        SDL_RWops *rw = SDL_RWFromConstMem(data.data(), data.size());
        SDL_Surface *s = rw ? IMG_Load_RW(rw, 1) : nullptr;
        if (s) {
          if (s->w > 0 && s->h > 0 && s->w <= 4096 && s->h <= 4096) {
            SDL_Surface *scaled = SDL_CreateRGBSurfaceWithFormat(
                0, 240, 220, 32, SDL_PIXELFORMAT_RGBA32);
            if (scaled) {
              SDL_FillRect(scaled, nullptr, 0);
              double scale = std::min(240.0 / s->w, 220.0 / s->h);
              SDL_Rect dst{int((240 - s->w * scale) / 2),
                           int((220 - s->h * scale) / 2), int(s->w * scale),
                           int(s->h * scale)};
              SDL_BlitScaled(s, nullptr, scaled, &dst);
              t = SDL_CreateTextureFromSurface(renderer, scaled);
              SDL_FreeSurface(scaled);
            }
          }
          SDL_FreeSurface(s);
        }
      }
      if (t)
        covers[coverid] = {t, frame};
      else {
        if (coverfailed.size() > 128)
          coverfailed.clear();
        coverfailed.insert(coverid);
      }
      coverid = 0;
    }
    while (covers.size() > 24) {
      auto old =
          std::min_element(covers.begin(), covers.end(), [](auto &a, auto &b) {
            return a.second.stamp < b.second.stamp;
          });
      SDL_DestroyTexture(old->second.texture);
      covers.erase(old);
    }
    if (screen != Screen::Grid || coverfuture.valid())
      return;
    int end = std::min<int>(roms.size(), (top + 2) * 4);
    for (int i = top * 4; i < end; i++) {
      auto &r = roms[i];
      auto it = covers.find(r.id);
      if (it != covers.end()) {
        it->second.stamp = frame;
        continue;
      }
      if (r.cover_url.empty() || coverfailed.count(r.id))
        continue;
      coverid = r.id;
      coverfuture =
          std::async(std::launch::async, [c = cfg, url = r.cover_url]() {
            try {
              return CoverResult{Client(c).cover(url), 0};
            } catch (const HttpError &e) {
              return CoverResult{{}, e.status};
            } catch (...) {
              return CoverResult{};
            }
          });
      break;
    }
  }
  void setup() {
    SwkbdConfig kb;
    if (R_FAILED(swkbdCreate(&kb, 0))) {
      message = "Keyboard unavailable. Set server_url in config.json.";
      return;
    }
    swkbdConfigMakePresetDefault(&kb);
    swkbdConfigSetHeaderText(&kb, "birdROMs server URL");
    swkbdConfigSetGuideText(&kb, "https://your-birdroms-server");
    swkbdConfigSetInitialText(&kb, cfg.server_url.c_str());
    swkbdConfigSetStringLenMax(&kb, 1023);
    char out[1024] = {};
    auto rc = swkbdShow(&kb, out, sizeof(out));
    swkbdClose(&kb);
    if (R_FAILED(rc))
      return;
    std::string url = out;
    while (!url.empty() && url.back() == '/')
      url.pop_back();
    if (url.rfind("https://", 0) != 0 && url.rfind("http://", 0) != 0) {
      message = "Enter a complete http:// or https:// URL.";
      return;
    }
    cfg.server_url = url;
    // Pairing drafts remain in memory. Persist only after approval delivers
    // a device token; cancellation and failures must leave SD unchanged.
    pair();
  }
  void press(u64 keys) {
    if (keys & HidNpadButton_Minus) {
      quit = true;
      return;
    }
    if (screen == Screen::Directory) {
      press_folders(keys);
      return;
    }
    if (keys & HidNpadButton_B) {
      if (screen == Screen::Download) {
        auto d = downloads.snapshot();
        if (d.running)
          downloads.cancel();
        else
          screen = download_return;
        return;
      }
      if (screen == Screen::Grid) {
        generation++;
        selecting_all = false;
        screen = Screen::Home;
        message.clear();
        return;
      }
      quit = true;
      return;
    }
    if (screen == Screen::Setup) {
      if (keys & HidNpadButton_A)
        setup();
      return;
    }
    if (screen == Screen::Pair) {
      if (keys & HidNpadButton_A && !message.empty()) {
        if (cfg.paired()) {
          try {
            save_paired_config(cfgpath, cfg);
            home();
          } catch (const std::exception &e) {
            message = e.what();
          }
        } else
          pair();
      }
      return;
    }
    if (screen == Screen::Download)
      return;
    if (keys & HidNpadButton_Plus) {
      if (!selection.items().empty()) {
        downloads.start(cfg, selection.items());
        download_return = screen;
        screen = Screen::Download;
      }
      return;
    }
    if (screen == Screen::Home) {
      if (keys & HidNpadButton_X && (job == Job::None || job == Job::Home)) {
        screen = Screen::Directory;
        folder_page = {};
        folder_focus = folder_top = 0;
        request_folder(cfg.rom_base, true);
        return;
      }
      int n = platforms.size() + playlists.size();
      if (keys & HidNpadButton_Up)
        homefocus = move_focus(homefocus, -1, n);
      if (keys & HidNpadButton_Down)
        homefocus = move_focus(homefocus, 1, n);
      if (keys & HidNpadButton_L)
        homefocus = move_focus(homefocus, -9, n);
      if (keys & HidNpadButton_R)
        homefocus = move_focus(homefocus, 9, n);
      if (keys & HidNpadButton_A && job == Job::None) {
        if (n)
          open();
        else
          home();
      }
      return;
    }
    if (job == Job::All)
      return;
    int n = roms.size();
    if (keys & HidNpadButton_Left)
      focus = move_focus(focus, -1, n);
    if (keys & HidNpadButton_Right)
      focus = move_focus(focus, 1, n);
    if (keys & HidNpadButton_Up)
      focus = move_focus(focus, -4, n);
    if (keys & HidNpadButton_Down)
      focus = move_focus(focus, 4, n);
    if (keys & HidNpadButton_L)
      focus = move_focus(focus, -8, n);
    if (keys & HidNpadButton_R)
      focus = move_focus(focus, 8, n);
    if (keys & HidNpadButton_A && n)
      selection.toggle(roms[focus]);
    if (keys & HidNpadButton_Y && n) {
      if (next) {
        selecting_all = true;
        more(true);
      } else
        selection.all(roms);
    }
    top = grid_top(focus, top);
    if (next && focus >= n - 8 && message.empty())
      more();
    if (keys & HidNpadButton_A && !message.empty() && next)
      more();
  }
  void draw_pair() {
    text("PAIR DEVICE", 570, 110, neon, big);
    text("Scan this QR code with your phone", 570, 175);
    text("and approve the device while signed in.", 570, 215);
    text(session.code, 570, 285, pink, big);
    text("Waiting for approval...", 570, 365, neon);
    text(session.qr_url, 570, 420, dim, small, 650);
    int elapsed =
        std::chrono::duration_cast<std::chrono::seconds>(Clock::now() - receipt)
            .count();
    text("Code expires in about " + std::to_string(std::max(0, 300 - elapsed)) +
             " seconds",
         570, 465, dim, small);
    if (session.qr_url != qrfor) {
      qrfor = session.qr_url;
      qr.resize(qrcodegen_BUFFER_LEN_MAX);
      std::vector<uint8_t> tmp(qrcodegen_BUFFER_LEN_MAX);
      qrvalid = qrcodegen_encodeText(
          qrfor.c_str(), tmp.data(), qr.data(), qrcodegen_Ecc_MEDIUM,
          qrcodegen_VERSION_MIN, qrcodegen_VERSION_MAX, qrcodegen_Mask_AUTO,
          true);
    }
    if (qrvalid) {
      int n = qrcodegen_getSize(qr.data()), scale = 400 / (n + 8);
      int edge = (n + 8) * scale;
      fill({255, 255, 255, 255}, {80, 130, edge, edge});
      for (int y = 0; y < n; y++)
        for (int x = 0; x < n; x++)
          if (qrcodegen_getModule(qr.data(), x, y))
            fill({0, 0, 0, 255},
                 {80 + (x + 4) * scale, 130 + (y + 4) * scale, scale, scale});
    }
  }
  void draw_home() {
    text("birdROMs", 40, 20, neon, big);
    text(std::to_string(selection.items().size()) + " selected", 980, 28, pink);
    std::vector<std::pair<std::string, int>> rows;
    rows.push_back({"PLAYLISTS", -1});
    for (size_t i = 0; i < playlists.size(); i++)
      rows.push_back({playlists[i].name + "  (" +
                          std::to_string(playlists[i].rom_count) + ")",
                      int(i)});
    rows.push_back({"PLATFORMS", -1});
    for (size_t i = 0; i < platforms.size(); i++)
      rows.push_back({platforms[i], int(playlists.size() + i)});
    int cur = 0;
    for (size_t i = 0; i < rows.size(); i++)
      if (rows[i].second == homefocus)
        cur = i;
    if (cur < hometop)
      hometop = cur;
    if (cur >= hometop + 10)
      hometop = cur - 9;
    for (int i = hometop; i < (int)rows.size() && i < hometop + 10; i++) {
      int y = 90 + (i - hometop) * 50;
      if (rows[i].second == -1)
        text(rows[i].first, 50, y, pink, small);
      else {
        bool selected = rows[i].second == homefocus;
        if (selected) {
          fill(panel, {35, y - 2, 1210, 46});
          fill(neon, {35, y - 2, 5, 46});
        }
        text(rows[i].first, 60, y, selected ? neon : dim);
      }
    }
    if (rows.size() == 2)
      text(job == Job::Home ? "Loading library..."
                            : "No platforms or playlists. A refreshes.",
           60, 210);
    text("A open    X ROM directory    B exit    L/R page    + download "
         "selected",
         40, 665, dim, small);
  }
  void draw_grid() {
    text(title, 40, 20, neon, big, 800);
    text(std::to_string(selection.items().size()) + " selected", 990, 28, pink);
    int end = std::min<int>(roms.size(), (top + 2) * 4);
    for (int i = top * 4; i < end; i++) {
      auto &r = roms[i];
      int x = 40 + (i % 4) * 305, y = 85 + (i / 4 - top) * 275;
      fill(panel, {x, y, 285, 260});
      if (i == focus) {
        outline(neon, {x - 3, y - 3, 291, 266});
        outline(neon, {x - 2, y - 2, 289, 264});
      }
      auto it = covers.find(r.id);
      if (it != covers.end()) {
        SDL_Rect d{x + 22, y + 7, 240, 220};
        SDL_RenderCopy(renderer, it->second.texture, nullptr, &d);
      } else
        text("NO COVER", x + 65, y + 95, dim, small);
      text(r.title, x + 10, y + 226, i == focus ? neon : dim, small, 265);
      if (selection.contains(r.id)) {
        fill(pink, {x + 245, y + 10, 30, 30});
        text("+", x + 251, y + 7, {255, 255, 255, 255});
      }
    }
    if (roms.empty())
      text(job == Job::Open ? "Loading..." : "No ROMs in this view", 45, 250);
    if (!roms.empty()) {
      auto &r = roms[focus];
      std::string info = r.platform + "  " + bytes(r.size_bytes);
      for (auto &t : r.tags)
        info += "  " + t;
      text(info, 40, 641, dim, small, 1050);
      text(std::to_string(focus + 1) + "/" + std::to_string(total), 1120, 641,
           dim, small, 150);
    }
    text("A select    Y all/none    L/R page    + download    B back", 40, 682,
         dim, small);
  }
  void draw_download() {
    auto d = downloads.snapshot();
    text("DOWNLOADS", 40, 25, neon, big);
    text(std::to_string(d.index) + " / " + std::to_string(d.count), 40, 120,
         pink, big);
    text(d.title, 40, 205, neon, normal);
    double ratio =
        d.total ? std::clamp(double(d.written) / d.total, 0.0, 1.0) : 0;
    fill(panel, {40, 300, 1200, 38});
    fill(neon, {40, 300, int(1200 * ratio), 38});
    text(std::to_string(int(ratio * 100)) + "%   " + bytes(d.written) + " / " +
             bytes(d.total) + "   " + bytes(d.speed) + "/s",
         40, 365);
    text("Completed " + std::to_string(d.done) + "   Skipped " +
             std::to_string(d.skipped) + "   Failed " +
             std::to_string(d.failed),
         40, 440);
    text(d.message, 40, 500, d.failed ? pink : dim, small);
    if (!d.last_error.empty())
      text(d.last_error, 40, 550, pink, small);
    text(d.running ? "B cancel downloads" : "B return to library", 40, 665, dim,
         small);
  }

public:
  App(Config c, std::string path)
      : cfg(std::move(c)), cfgpath(std::move(path)) {}
  ~App() {
    downloads.cancel();
    if (future.valid())
      future.wait();
    if (coverfuture.valid())
      coverfuture.wait();
    if (folder_future.valid())
      folder_future.wait();
    for (auto &c : covers)
      SDL_DestroyTexture(c.second.texture);
    if (big)
      TTF_CloseFont(big);
    if (normal)
      TTF_CloseFont(normal);
    if (small)
      TTF_CloseFont(small);
    if (renderer)
      SDL_DestroyRenderer(renderer);
    if (window)
      SDL_DestroyWindow(window);
    IMG_Quit();
    TTF_Quit();
    SDL_Quit();
  }
  int run() {
    if (SDL_Init(SDL_INIT_VIDEO | SDL_INIT_GAMECONTROLLER) != 0 ||
        TTF_Init() != 0)
      return 1;
    IMG_Init(IMG_INIT_PNG | IMG_INIT_JPG);
    window = SDL_CreateWindow("birdROMs", 0, 0, 1280, 720, 0);
    if (!window)
      return 1;
    renderer = SDL_CreateRenderer(
        window, -1, SDL_RENDERER_ACCELERATED | SDL_RENDERER_PRESENTVSYNC);
    if (!renderer)
      return 1;
    SDL_RenderSetLogicalSize(renderer, 1280, 720);
    normal = TTF_OpenFont("romfs:/font.ttf", 26);
    small = TTF_OpenFont("romfs:/font.ttf", 20);
    big = TTF_OpenFont("romfs:/font.ttf", 38);
    if (!normal || !small || !big)
      return 1;
    padConfigureInput(1, HidNpadStyleSet_NpadStandard);
    padInitializeDefault(&pad);
    if (cfg.server_url.empty())
      screen = Screen::Setup;
    else if (!cfg.paired())
      pair();
    else
      home();
    while (!quit && appletMainLoop()) {
      frame++;
      SDL_Event e;
      while (SDL_PollEvent(&e))
        if (e.type == SDL_QUIT)
          quit = true;
      padUpdate(&pad);
      u64 keys = padGetButtonsDown(&pad), held = padGetButtons(&pad);
      HidAnalogStickState stick = padGetStickPos(&pad, 0);
      u64 direction = held & (HidNpadButton_Up | HidNpadButton_Down |
                              HidNpadButton_Left | HidNpadButton_Right);
      if (stick.x > 16000)
        direction |= HidNpadButton_Right;
      if (stick.x < -16000)
        direction |= HidNpadButton_Left;
      if (stick.y > 16000)
        direction |= HidNpadButton_Up;
      if (stick.y < -16000)
        direction |= HidNpadButton_Down;
      uint64_t now = SDL_GetTicks64();
      if (direction != lastdirection) {
        keys |= direction;
        repeatAt = now + 350;
        lastdirection = direction;
      } else if (direction && now >= repeatAt) {
        keys |= direction;
        repeatAt = now + 110;
      }
      collect();
      collect_folders();
      if (screen == Screen::Download && downloads.snapshot().finished &&
          downloads.snapshot().unauthorized) {
        generation++;
        cfg.device_token.clear();
        selection.clear();
        pair();
      }
      if (keys)
        press(keys);
      if (screen == Screen::Pair) {
        if (!session.code.empty() &&
            Clock::now() - receipt > std::chrono::minutes(5) &&
            job == Job::None) {
          if (restarts++ == 0)
            session = {};
          else
            message = "Pairing code expired. A retries.";
        }
        request_pair();
      }
      cover_update();
      SDL_SetRenderDrawColor(renderer, bg.r, bg.g, bg.b, 255);
      SDL_RenderClear(renderer);
      if (screen == Screen::Home)
        draw_home();
      else if (screen == Screen::Grid)
        draw_grid();
      else if (screen == Screen::Download)
        draw_download();
      else if (screen == Screen::Directory)
        draw_folders();
      else if (screen == Screen::Setup) {
        text("birdROMs", 50, 35, neon, big);
        text("Configure your birdROMs server", 50, 180, neon, big);
        text("A enter server URL    B exit", 50, 300);
        text("ROM directory and platform mappings are editable in config.json.",
             50, 365, dim, small);
      } else {
        if (session.code.empty())
          text("Contacting server...", 50, 200, neon);
        else
          draw_pair();
        text("B cancel    A retry after an error", 40, 665, dim, small);
      }
      if (job != Job::None && screen != Screen::Pair &&
          screen != Screen::Directory)
        text(job == Job::All ? "Selecting entire view..." : "Loading...", 950,
             610, neon, small);
      if (!message.empty())
        text(message, 40, 610, pink, small, 1180);
      SDL_RenderPresent(renderer);
      SDL_Delay(16);
    }
    return 0;
  }
};
} // namespace
int run_ui(Config cfg, const std::string &path) {
  return App(std::move(cfg), path).run();
}
} // namespace bird
