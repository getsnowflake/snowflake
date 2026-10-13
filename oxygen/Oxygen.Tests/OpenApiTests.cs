using FluentAssertions;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.Extensions.DependencyInjection;
using Swashbuckle.AspNetCore.Swagger;
using Xunit;

namespace Oxygen.Tests;

public class OpenApiTests
{
    [Fact]
    public void GeneratedContractIncludesAnnotationsAndPublicHealthSecurity()
    {
        using var factory = new WebApplicationFactory<Program>().WithWebHostBuilder(builder =>
            builder.UseEnvironment("Production"));
        var spec = factory.Services.GetRequiredService<ISwaggerProvider>().GetSwagger("v1");
        var loan = spec.Paths["/loan"].Operations.Values.Single();
        loan.OperationId.Should().Be("applyForLoan");
        loan.Summary.Should().Be("Apply for a loan");
        loan.RequestBody.Description.Should().Contain("USD");
        loan.Responses.Keys.Should().Contain(new[] { "200", "400", "401" });
        var health = spec.Paths["/loan/health"].Operations.Values.Single();
        health.Security.Should().ContainSingle().Which.Should().BeEmpty();
    }
}
