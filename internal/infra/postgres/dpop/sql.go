package dpop

const qInsertJTI = `INSERT INTO guliangiam.dpop_replay (jti) VALUES ($1) ON CONFLICT DO NOTHING`
