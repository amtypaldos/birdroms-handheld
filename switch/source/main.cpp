#include <switch.h>
#include <SDL.h>
int main(int,char**) {
 if(SDL_Init(SDL_INIT_VIDEO|SDL_INIT_GAMECONTROLLER)) return 1;
 SDL_Window* w=SDL_CreateWindow("birdROMs",0,0,1280,720,0);
 SDL_Renderer* r=w?SDL_CreateRenderer(w,-1,SDL_RENDERER_ACCELERATED):nullptr;
 if(!r){SDL_DestroyWindow(w);SDL_Quit();return 1;}
 PadState pad;padConfigureInput(1,HidNpadStyleSet_NpadStandard);padInitializeDefault(&pad);
 while(appletMainLoop()) { padUpdate(&pad); if(padGetButtonsDown(&pad)&HidNpadButton_Plus)break;
 SDL_Event e;bool quit=false;while(SDL_PollEvent(&e))if(e.type==SDL_QUIT)quit=true;if(quit)break;
 SDL_SetRenderDrawColor(r,11,13,23,255);SDL_RenderClear(r);
 SDL_SetRenderDrawColor(r,0,240,255,255);SDL_Rect panel{80,80,1120,560};SDL_RenderDrawRect(r,&panel);SDL_RenderPresent(r);SDL_Delay(16);}
 SDL_DestroyRenderer(r);SDL_DestroyWindow(w);SDL_Quit();return 0;
}
