#pragma once
#include <string>
#include <vector>
#include <map>
#include <atomic>
#include <functional>
#include <cstdint>
namespace bird {
struct Rom {int id=0;std::string title,platform,extension,filename,cover_url;int64_t size_bytes=0;std::vector<std::string> tags;};
struct Page {int count=0;bool next=false;std::vector<Rom> results;};
struct Playlist {int id=0;std::string name;int rom_count=0;};
struct Config {int version=1;std::string server_url,device_token;std::string rom_base="sdmc:/retroarch/roms/";std::map<std::string,std::string> platform_dirs;bool paired()const{return !server_url.empty()&&!device_token.empty();}};
struct PairSession {std::string code,qr_url,expires_at;};
enum class PollStatus {Pending,Approved,Expired,RateLimited};
struct PollResult {PollStatus status;std::string token;};
}
