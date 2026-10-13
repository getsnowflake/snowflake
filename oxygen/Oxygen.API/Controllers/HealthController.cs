using Microsoft.AspNetCore.Mvc;
using Microsoft.AspNetCore.Authorization;
using Microsoft.EntityFrameworkCore;
using Oxygen.DTO.Response;
using Oxygen.Infrastructure;
using Swashbuckle.AspNetCore.Annotations;

namespace Oxygen.API.Controllers;

[ApiController]
[AllowAnonymous]
[Route("loan")]
[Tags("Health")]
public class HealthController(ApplicationDbContext dbContext) : ControllerBase
{
    [SwaggerOperation(
        OperationId = "oxygenHealth",
        Summary    = "Health check",
        Description = "Liveness/readiness check (database)"
    )]
    [SwaggerResponse(200, "Service healthy", typeof(HealthResponse))]
    [SwaggerResponse(503, "Service unhealthy", typeof(HealthResponse))]
    [HttpGet("health")]
    public async Task<IActionResult> Health()
    {
        var components = new Dictionary<string, string>();
        var allHealthy = true;

        // Database check
        try
        {
            var canConnect = await dbContext.Database.CanConnectAsync();
            if (canConnect)
            {
                components["database"] = "ok";
            }
            else
            {
                components["database"] = "cannot connect";
                allHealthy = false;
            }
        }
        catch (Exception e)
        {
            components["database"] = e.Message;
            allHealthy = false;
        }

        var response = new
        {
            status = allHealthy ? "healthy" : "unhealthy",
            components
        };

        return allHealthy
            ? Ok(response)
            : StatusCode(503, response);
    }
}
