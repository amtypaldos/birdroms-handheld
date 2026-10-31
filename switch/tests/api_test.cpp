#include "api/parsing.hpp"
#include <cassert>
#include <iostream>
using namespace bird;
int main(){using namespace parsing;
 auto j=Json::parse(R"({"count":25,"next":"https://server/api/roms/?page=2","previous":null,"results":[{"id":3,"title":"Game","platform":"NES","extension":".nes","filename":"Game (USA).nes","size_bytes":null,"cover_url":null,"tags":["USA"]}]})");
 auto p=page(j);assert(p.next&&p.count==25&&p.results.size()==1);assert(p.results[0].size_bytes==0&&p.results[0].filename=="Game (USA).nes"&&p.results[0].tags[0]=="USA");
 j["next"]=nullptr;assert(!page(j).next);
 assert(poll(202,Json{{"device_token","must ignore"}}).token.empty());assert(poll(410).status==PollStatus::Expired);assert(poll(429).status==PollStatus::RateLimited);
 assert(poll(200,Json{{"device_token","approved"}}).token=="approved");
 bool bad=false;try{poll(200,Json{{"device_token",""}});}catch(...){bad=true;}assert(bad);
 bad=false;try{rom(Json{{"id",0}});}catch(...){bad=true;}assert(bad);
 bad=false;try{session(Json{{"code","ABC"}});}catch(...){bad=true;}assert(bad);
 bad=false;try{session(Json{{"code","ABC"},{"qr_url","https://server/pair"},{"expires_at","invalid"}});}catch(...){bad=true;}assert(bad);
 bad=false;try{poll(200,Json{{"device_token","bad\nheader"}});}catch(...){bad=true;}assert(bad);
 auto s=session(Json{{"code","ABC"},{"qr_url","https://server/pair?code=ABC"},{"expires_at","2026-10-31T00:00:00Z"}});assert(s.code=="ABC");
 std::cout<<"API JSON/pagination/pairing tests passed\n";
}
