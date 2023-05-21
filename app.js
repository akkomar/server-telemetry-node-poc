const express = require('express')

const app = express()
// create your mozlog instance
const mozlog = require('mozlog')({
  app: 'fxa-oauth-server',
  level: 'verbose', //default is INFO
  fmt: 'pretty', //default is 'heka'
  uncaught: 'exit', // default is 'log', also available as 'ignore'
  debug: true, //default is false
  stream: process.stderr //default is process.stdout
});
const log = mozlog('routes.client.register');

app.get('/', (req, res) => {
  res.send('Hello World!')
})

app.listen(3000, () => {
  console.log('Example app listening on port 3000!')
})

setInterval(() => {
    console.log('Logging something every second')
    log.info('gleanServerEvent', '{//serialized ping goes here}')
  }, 1000)