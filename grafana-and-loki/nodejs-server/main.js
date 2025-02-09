const winston = require("winston");

const serviceName = process.env.SERVICE_NAME || "NONE";
const containerID = process.env.HOSTNAME || "NONE";

const logger = winston.createLogger({
  level: "info",
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format((info) => {
      info.service = serviceName;
      info.containerID = containerID;
      return info;
    })(),
    winston.format.json()
  ),
  transports: [
    new winston.transports.Console(),
    new winston.transports.File({
      filename: `/var/log/loki/${serviceName}.log`,
    }),
  ],
});

async function sleep(ms) {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

async function main() {
  while (true) {
    logger.info({ message: "Hello, World!" });

    await sleep(5000);
  }
}

main();
