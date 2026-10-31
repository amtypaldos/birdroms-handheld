#include "ui/image_bounds.hpp"
#include <cassert>
#include <vector>
using Data = std::vector<unsigned char>;
Data png(unsigned width, unsigned height) {
    Data out{137,80,78,71,13,10,26,10,0,0,0,13,'I','H','D','R'};
    for(unsigned value:{width,height})
        for(int shift:{24,16,8,0}) out.push_back(value >> shift);
    out.insert(out.end(),{8,2,0,0,0,0,0,0,0});
    return out;
}
Data jpeg(unsigned width, unsigned height) {
    // A dimension-bearing baseline JPEG header through its first scan header.
    return {0xff,0xd8,0xff,0xe0,0,4,'J','F',0xff,0xc0,0,11,8,
            static_cast<unsigned char>(height>>8),static_cast<unsigned char>(height),
            static_cast<unsigned char>(width>>8),static_cast<unsigned char>(width),
            1,1,0x11,0,0xff,0xda,0,8,1,1,0,0,63,0};
}
int main() {
    using bird::cover_dimensions_safe;
    assert(cover_dimensions_safe(png(240,220)));
    assert(cover_dimensions_safe(png(2048,2048)));
    assert(!cover_dimensions_safe(png(2049,2048)));
    assert(!cover_dimensions_safe(png(4097,1)));
    assert(!cover_dimensions_safe(png(1,0)));
    assert(!cover_dimensions_safe(png(0xffffffff,0xffffffff)));
    auto p=png(100,100);p.resize(32);assert(!cover_dimensions_safe(p));
    p=png(100,100);p[12]='X';assert(!cover_dimensions_safe(p));
    assert(cover_dimensions_safe(jpeg(240,220)));
    assert(cover_dimensions_safe(jpeg(4096,1024)));
    assert(!cover_dimensions_safe(jpeg(4096,1025)));
    assert(!cover_dimensions_safe(jpeg(65535,65535)));
    assert(!cover_dimensions_safe(jpeg(0,100)));
    auto j=jpeg(100,100);j.pop_back();assert(!cover_dimensions_safe(j));
    j=jpeg(100,100);j[10]=0xff;assert(!cover_dimensions_safe(j));
    j=jpeg(100,100);j.erase(j.begin()+8,j.begin()+21);assert(!cover_dimensions_safe(j));
    assert(!cover_dimensions_safe({}));
    assert(!cover_dimensions_safe({'R','I','F','F',0,0,0,0,'W','E','B','P'}));
}
