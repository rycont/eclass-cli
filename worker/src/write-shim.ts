// Invocation-local filesystem for the existing CLI's disk-writing commands.
// Never exposes the credential/session store, which uses separate KV hooks.
export const MAX_FILE = 16 * 1024 * 1024;
export const MAX_TOTAL = 64 * 1024 * 1024;
export function writeShim(stdout: (text: string) => void, stderr: (text: string) => void) {
  const files = new Map<string, Uint8Array>();
  const dirs = new Set(["/", "/work"]);
  const fds = new Map<number, {path:string;pos:number}>(); let fd = 3;
  let total = 0;
  let failed: unknown = null;
  const error = (code:string) => Object.assign(new Error(code), {code});
  const path = (p:string) => {
    if (p.includes("\0") || p.split("/").includes("..")) throw error("EACCES");
    const out = (p.startsWith("/") ? p : "/work/" + p).replace(/\/+/g,"/").replace(/\/$/,"") || "/";
    if (out !== "/" && out !== "/work" && !out.startsWith("/work/")) throw error("EACCES");
    return out;
  };
  const parent = (p:string) => p.slice(0,p.lastIndexOf("/")) || "/";
  const stat = (p:string) => {
    const directory = dirs.has(p), data=files.get(p);
    if (!directory && !data) throw error("ENOENT");
    return {dev:0,ino:1,mode:directory?0o40700:0o100600,nlink:1,uid:0,gid:0,rdev:0,
      size:data?.length??0,blksize:4096,blocks:0,atimeMs:0,mtimeMs:0,ctimeMs:0,isDirectory:()=>directory};
  };
  const cb = (callback:any, fn:()=>any) => {try {callback(null,fn());} catch(e) {callback(e);}};
  const write = (n:number,buf:Uint8Array,offset:number,length:number,position:number|null) => {
    if(n===1 || n===2){(n===1?stdout:stderr)(new TextDecoder().decode(buf.subarray(offset,offset+length)));return length;}
    const f=fds.get(n); if(!f)throw error("EBADF");
    const old=files.get(f.path)!; const at=position??f.pos, size=Math.max(old.length,at+length);
    if(at<0 || size>MAX_FILE || total-old.length+size>MAX_TOTAL){failed=error("EFBIG");throw failed;}
    const data=new Uint8Array(size);data.set(old);data.set(buf.subarray(offset,offset+length),at);
    files.set(f.path,data);total+=size-old.length;if(position===null)f.pos=at+length;return length;
  };
  const fs:any = {
    constants:{O_WRONLY:1,O_RDWR:2,O_CREAT:64,O_TRUNC:512,O_APPEND:1024,O_EXCL:128,O_DIRECTORY:65536},
    writeSync:(n:number,b:Uint8Array)=>write(n,b,0,b.length,null),
    write:(n:number,b:Uint8Array,o:number,l:number,p:number|null,c:any)=>cb(c,()=>write(n,b,o,l,p)),
    open:(p:string,flags:number,_mode:number,c:any)=>cb(c,()=>{p=path(p);if(!dirs.has(parent(p)))throw error("ENOENT");if(!files.has(p)&&!dirs.has(p)){if(!(flags&64))throw error("ENOENT");files.set(p,new Uint8Array());}else if(flags&128)throw error("EEXIST");if(flags&512 && files.has(p)){total-=files.get(p)!.length;files.set(p,new Uint8Array());}const n=fd++;fds.set(n,{path:p,pos:flags&1024?files.get(p)?.length??0:0});return n;}),
    close:(n:number,c:any)=>cb(c,()=>{if(!fds.delete(n))throw error("EBADF");}),
    read:(n:number,b:Uint8Array,o:number,l:number,p:number|null,c:any)=>cb(c,()=>{const f=fds.get(n);if(!f)throw error("EBADF");const d=files.get(f.path);if(!d)throw error("EISDIR");const at=p??f.pos,count=Math.max(0,Math.min(l,d.length-at));b.set(d.subarray(at,at+count),o);if(p===null)f.pos+=count;return count;}),
    mkdir:(p:string,_m:number,c:any)=>cb(c,()=>{p=path(p);if(dirs.has(p)||files.has(p))throw error("EEXIST");if(!dirs.has(parent(p)))throw error("ENOENT");dirs.add(p);}),
    stat:(p:string,c:any)=>cb(c,()=>stat(path(p))),
    lstat:(p:string,c:any)=>cb(c,()=>stat(path(p))),
    fstat:(n:number,c:any)=>cb(c,()=>{const f=fds.get(n);if(!f)throw error("EBADF");return stat(f.path);}),
    readdir:(p:string,c:any)=>cb(c,()=>{p=path(p);if(!dirs.has(p))throw error("ENOENT");return [...files.keys(),...dirs].filter(x=>parent(x)===p&&x!==p).map(x=>x.slice(p.length+1));}),
    unlink:(p:string,c:any)=>cb(c,()=>{p=path(p);const d=files.get(p);if(!d)throw error("ENOENT");total-=d.length;files.delete(p);}),
    rename:(a:string,b:string,c:any)=>cb(c,()=>{a=path(a);b=path(b);if(!dirs.has(parent(b)))throw error("ENOENT");const data=files.get(a);if(!data)throw error("ENOENT");if(dirs.has(b))throw error("EISDIR");total-=files.get(b)?.length??0;files.delete(a);files.set(b,data);for(const f of fds.values())if(f.path===a)f.path=b;}),
    fsync:(_n:number,c:any)=>c(null),
    chmod:(p:string,_m:number,c:any)=>cb(c,()=>stat(path(p))),
    fchmod:(n:number,_m:number,c:any)=>cb(c,()=>{if(!fds.has(n))throw error("EBADF");}),
  };
  for (const name of ["chown","fchown","ftruncate","lchown","link","readlink","rmdir","symlink","truncate","utimes"])fs[name]=(...a:any[])=>a[a.length-1](error("ENOSYS"));
  return {fs, files, failed:()=>failed, process:{cwd:()=>"/work",chdir:(p:string)=>{if(path(p)!=="/work")throw error("EACCES");},getuid:()=>0,getgid:()=>0,geteuid:()=>0,getegid:()=>0,getgroups:()=>[],pid:1,ppid:0,umask:()=>0}, path:{resolve:(...parts:string[])=>path(parts.join("/"))}};
}
