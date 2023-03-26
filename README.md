# RequestLimiter
RequestLimiter is a simple Go application that updates a counter in a database based on incoming GET and POST requests. The application includes a rate limiter that blocks requests if the rate of POST requests exceeds a specified threshold within a given time period. If the rate limit is exceeded or in case the update-time has passed, the counter update is temporarily cached until the time period has elapsed.


## Usage
To run the application, execute the following command in your terminal:
```sh
go run .\server.go -threshold=<max_requests> -ttl=<time_to_life> -update=<update_time>
```
- **max_requests:** the maximum number of requests allowed in the time period.
- **time_to_life:** the length of the time period in milliseconds.
 - **update_time:** the length of the interval in milliseconds at which the counter is updated if no requests are received.

## API Endpoints
The application provides the following API endpoints:
- **GET /counter**

Returns the current value of the counter:
```json
{
  "count": <counter>
}
```
- **POST /counter**

Updates the counter and returns its current value as above. If the rate limit is exceeded, the request is blocked and the following response is returned:
```json
{
  "count": <counter>,
  "isBlocked": true
}
```
## Architecture
The RequestLimiter app architecture consists of several files, including server.go, counter-service.go, db-service.go, and a test file. The server accepts HTTP GET requests and returns the current counter value. 

For HTTP POST requests (see the schematic figure), the limiter service uses a limiter struct initialized by the Start function, which sets up relevant timers and channels for handling incoming requests. The counter-service listens for requests using a separate Go routine. If the incoming request rate exceeds a certain threshold, the limiter signals the blockedChann, resulting in a delayed response from the server. If the incoming request rate is below the threshold, the server responds immediately with an updated counter value. Overall, the architecture of the limiter app is designed to ensure that incoming requests are handled efficiently and consistently, while preventing overwhelming the system with too many requests.

![Schematic description of the program's architecture.](/public/assets/limit-diagram.jpg)

## Testing
To run the tests for this application, execute the following command in your terminal:
```sh
go test ./.... -race 
```
This command runs all the tests in the current and all subdirectories with the -race flag, which detects race conditions. The tests ensure that the application works as expected and that no issues occur during implementation.

## Conclusion
RequestLimiter is a simple yet efficient Go application that updates a counter in a database based on incoming GET and POST requests. The rate limiter ensures that the application handles incoming requests efficiently, preventing the system from becoming overwhelmed by too many requests.

If you have any questions, please feel free to reach out to the author of this code.


