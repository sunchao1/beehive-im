// Mongo 初始化：应用账号 beehive/111111，库 chat（与 conf/*.xml 一致）

db = db.getSiblingDB('chat');

db.createUser({
  user: 'beehive',
  pwd: '111111',
  roles: [{ role: 'readWrite', db: 'chat' }]
});

db.RoomMesg.createIndex({ rid: 1 });
db.RoomMesg.createIndex({ uid: 1 });
db.RoomMesg.createIndex({ rid: 1, uid: 1 });

db.RoomBlacklist.createIndex({ rid: 1 });
db.RoomBlacklist.createIndex({ uid: 1 });
db.RoomBlacklist.createIndex({ rid: 1, uid: 1 }, { unique: true });
