using Microsoft.OpenApi.Models;
using Microsoft.OpenApi.Extensions;
using Swashbuckle.AspNetCore.Swagger;
using Swashbuckle.AspNetCore.SwaggerGen;
using Microsoft.AspNetCore.Authorization;

namespace Oxygen.API.Extensions;

public static class SwaggerExtensions
{
    public static void SaveSwaggerJson(this IServiceProvider provider, string outputPath = "../openapi.json")
    {
        ISwaggerProvider sw = provider.GetRequiredService<ISwaggerProvider>();
        OpenApiDocument doc = sw.GetSwagger("v1", null, "/");
        string swaggerFile = doc.SerializeAsJson(Microsoft.OpenApi.OpenApiSpecVersion.OpenApi3_0);
        File.WriteAllText(outputPath, swaggerFile);
    }
}

public class AnonymousOperationFilter : IOperationFilter
{
    public void Apply(OpenApiOperation operation, OperationFilterContext context)
    {
        if (context.ApiDescription.ActionDescriptor.EndpointMetadata.OfType<IAllowAnonymous>().Any())
            operation.Security = new List<OpenApiSecurityRequirement> { new() };
    }
}
