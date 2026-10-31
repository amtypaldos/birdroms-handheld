#pragma once
#include "types.hpp"
#include <nlohmann/json.hpp>
#include <stdexcept>
#include <sstream>
#include <iomanip>
#include <ctime>
namespace bird::parsing {
using Json=nlohmann::json;
inline std::string text(const Json& j,const char* key) {
 auto it=j.find(key);return it==j.end()||it->is_null()?std::string{}:it->get<std::string>();
}
inline Rom rom(const Json& j) {
 Rom r;r.id=j.at("id").get<int>();if(r.id<=0)throw std::runtime_error("Invalid ROM identifier");
 r.title=text(j,"title");r.platform=text(j,"platform");r.extension=text(j,"extension");
 r.filename=text(j,"filename");r.cover_url=text(j,"cover_url");
 if(j.contains("size_bytes")&&!j["size_bytes"].is_null())r.size_bytes=j["size_bytes"].get<int64_t>();
 if(r.size_bytes<0)throw std::runtime_error("Invalid ROM size");
 if(j.contains("tags")&&!j["tags"].is_null())r.tags=j["tags"].get<std::vector<std::string>>();
 return r;
}
inline std::vector<Rom> rom_list(const Json& j) {
 if(!j.is_array())throw std::runtime_error("Expected ROM list");
 std::vector<Rom> out;out.reserve(j.size());for(const auto& v:j)out.push_back(rom(v));return out;
}
inline Page page(const Json& j) {
 Page p;p.count=j.at("count").get<int>();if(p.count<0)throw std::runtime_error("Invalid ROM count");
 p.next=j.contains("next")&&!j["next"].is_null()&&!j["next"].get<std::string>().empty();
 p.results=rom_list(j.at("results"));return p;
}
inline PairSession session(const Json& j) {
 PairSession s{text(j,"code"),text(j,"qr_url"),text(j,"expires_at")};
 if(s.code.empty()||s.qr_url.empty()||s.expires_at.empty())throw std::runtime_error("Pairing response missing fields");
 std::tm date{};std::istringstream input(s.expires_at.substr(0,19));
 input >> std::get_time(&date,"%Y-%m-%dT%H:%M:%S");
 if(input.fail()||s.expires_at.size()<20)throw std::runtime_error("Invalid pairing expiry timestamp");
 return s;
}
inline PollResult poll(long status,const Json& j=Json{}) {
 if(status==202)return {PollStatus::Pending,{}};
 if(status==410)return {PollStatus::Expired,{}};
 if(status==429)return {PollStatus::RateLimited,{}};
 if(status!=200)throw std::runtime_error("Unexpected pairing status");
 auto token=text(j,"device_token");if(token.empty())throw std::runtime_error("Approval response missing device token");
 for(unsigned char c:token)if(c<=32||c==127)throw std::runtime_error("Invalid pairing device token");
 return {PollStatus::Approved,token};
}
}
