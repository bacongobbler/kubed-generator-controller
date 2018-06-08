require 'sinatra'

set :bind, '0.0.0.0'
set :port, ENV['PORT']

get '/' do
  'Hello World, I\'m a Ruby Sinatra app!'
end
