import Vapor

let drop = Droplet()
drop.get("/") { _ in
  return "Hello World, I'm a Swift Vapor app!"
}
drop.run()

