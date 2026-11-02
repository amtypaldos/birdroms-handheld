#include "ui/update_policy.hpp"
#include <cassert>
#include <iostream>
int main(){using namespace bird;UpdatePolicy policy;UpdateState state;
 assert(policy.next(state,true)==UpdateAction::None);
 state.checked=true;state.available=true;assert(policy.next(state,false)==UpdateAction::None);assert(policy.next(state,true)==UpdateAction::Download);
 state.running=true;assert(policy.next(state,true)==UpdateAction::None);
 state.running=false;state.ready=true;assert(policy.next(state,true)==UpdateAction::Install);assert(policy.next(state,false)==UpdateAction::None);
 policy.skip();assert(policy.next(state,true)==UpdateAction::None);
 policy.retry();assert(policy.next(state,true)==UpdateAction::Install);
 state.ready=false;state.cancelled=true;assert(policy.next(state,true)==UpdateAction::None);
 state.cancelled=false;state.available=false;assert(policy.next(state,true)==UpdateAction::None);
 std::cout<<"Update UI policy tests passed (background/cancel/retry/install)\n";}
