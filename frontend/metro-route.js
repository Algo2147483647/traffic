/* Exact directional track geometry for a station-to-station itinerary leg. */
(function(root){
  function points(line,from,to){
    const a=line.stations.indexOf(from),b=line.stations.indexOf(to),n=line.stations.length;
    if(a<0||b<0||a===b||!line.stop_chainages)return [];
    const reverse=line.circular?(a-b+n)%n<(b-a+n)%n:b<a;
    const track=reverse&&line.circular?line.reverse_track:line.track;
    if(!track?.length)return [];
    const chain=[0];for(let i=1;i<track.length;i++)chain.push(chain[i-1]+Math.hypot(track[i].x-track[i-1].x,track[i].y-track[i-1].y));
    const total=chain.at(-1),direction=reverse?1:0,start=line.stop_chainages[a][direction];
    let end=line.stop_chainages[b][direction];if(end<start)end+=total;
    const sample=d=>{d=((d%total)+total)%total;let i=1;while(i<chain.length-1&&chain[i]<d)i++;const f=(d-chain[i-1])/(chain[i]-chain[i-1]);return {x:track[i-1].x+(track[i].x-track[i-1].x)*f,y:track[i-1].y+(track[i].y-track[i-1].y)*f};};
    const result=[sample(start)];for(let lap=0;lap<=1;lap++)for(let i=1;i<chain.length;i++){const d=chain[i]+lap*total;if(d>start&&d<end)result.push(track[i]);}result.push(sample(end));return result;
  }
  const api={points,status:(index,step)=>index<step?'done':index===step?'current':'next'};
  if(typeof module!=='undefined')module.exports=api;else root.MetroRoute=api;
})(globalThis);
