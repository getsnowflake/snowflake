using System.Text;
using System.Text.Json.Serialization;
using Oxygen.API.Extensions;
using Oxygen.API.Filters;
using Oxygen.API.Logging;
using Oxygen.API.Middleware;
using Oxygen.Infrastructure;
using Oxygen.Infrastructure.Adapters;
using Oxygen.Infrastructure.Extensions;
using Oxygen.Repository;
using Oxygen.Service;
using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.IdentityModel.Tokens;
using Microsoft.OpenApi.Models;
using Prometheus;
using Serilog;

var builder = WebApplication.CreateBuilder(args);

builder.Host.UseSerilog((ctx, lc) =>
    SerilogConfig.Configure(lc, ctx.HostingEnvironment.IsDevelopment()));

// Add services to the container.
// Learn more about configuring Swagger/OpenAPI at https://aka.ms/aspnetcore/swashbuckle
builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen(options =>
{
    options.EnableAnnotations();
    options.OperationFilter<AnonymousOperationFilter>();
        
    options.SwaggerDoc("v1", new OpenApiInfo
    {
        Title = "Snowflake API Reference",
        Version = "v1",
        Description = "The Snowflake API is organized around REST. This API has predictable resource-oriented URLs, accepts JSON-encoded request bodies, returns JSON-encoded responses, and uses standard HTTP response codes, authentication, and verbs.",
        Contact = new OpenApiContact
        {
            Name = "GitHub",
            Url = new Uri("https://github.com/getsnowflake/snowflake/issues")
        },
        License = new OpenApiLicense
        {
            Name = "GNU General Public License v3.0",
            Url = new Uri("https://www.gnu.org/licenses/gpl-3.0")
        },
    });
    
    options.AddServer(new OpenApiServer { Url = "https://snowflake.snowye.dev" });

    options.AddSecurityDefinition("Bearer", new OpenApiSecurityScheme
    {
        Name = "Authorization",
        Type = SecuritySchemeType.Http,
        Scheme = "bearer",
        BearerFormat = "JWT",
        Description =
            "Enter 'Bearer' [space] and your token in the text input below.\n\nExample: 'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...'"
    });

    options.AddSecurityRequirement(new OpenApiSecurityRequirement
    {
        {
            new OpenApiSecurityScheme
            {
                Reference = new OpenApiReference
                {
                    Type = ReferenceType.SecurityScheme,
                    Id = "Bearer"
                }
            },
            Array.Empty<string>()
        }
    });
    
    options.IncludeXmlComments(Path.Combine(AppContext.BaseDirectory, "Oxygen.API.xml"));
    options.IncludeXmlComments(Path.Combine(AppContext.BaseDirectory, "Oxygen.DTO.xml"));
});
builder.Services.AddSingleton<RateLimitFilter>();
builder.Services.AddControllers(options =>
    {
        options.Filters.Add<ValidationFilter>();
    })
    .AddJsonOptions(options => { options.JsonSerializerOptions.Converters.Add(new JsonStringEnumConverter()); });
builder.Services.AddDbContext<ApplicationDbContext>();
builder.Services.AddCreditScoreAdapter();

builder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme)
    .AddJwtBearer(options =>
    {
        var jwtSecret = builder.Configuration["Security:Jwt:SecretKey"];

        if (string.IsNullOrWhiteSpace(jwtSecret) || jwtSecret == "change-me-in-production-jwt-secret")
        {
            throw new InvalidOperationException("Security:Jwt:SecretKey is set to insecure default 'change-me-in-production-jwt-secret'. Set a real secret via environment variable.");
        }

        options.TokenValidationParameters = new TokenValidationParameters
        {
            ValidateIssuer = true,
            ValidIssuer = "snowflake",
            ValidateAudience = false,
            ValidateIssuerSigningKey = true,
            IssuerSigningKey = new SymmetricSecurityKey(Encoding.UTF8.GetBytes(jwtSecret))
        };

        options.Events = new JwtBearerEvents
        {
            OnAuthenticationFailed = context =>
            {
                Log.Warning("Authentication failed: {Message}", context.Exception.Message);
                return Task.CompletedTask;
            }
        };
    });

// Repositories
builder.Services.AddScoped<ILoanRepository, LoanRepository>();

// Adapters
builder.Services.AddScoped<IUsersGRPCAdapter, UsersGRPCAdapter>();

// Services
builder.Services.AddScoped<ILoanService, LoanService>();

var app = builder.Build();

// Prometheus: measure RED metrics for every request. Placed first so it is the
// outermost middleware and observes the final status of every response —
// including responses written by the exception handler below. Served on the
// HTTP port at /metrics (not proxied through nginx).
app.UseHttpMetrics();

app.UseMiddleware<ExceptionHandlingMiddleware>();

// Configure the HTTP request pipeline.
if (args.Contains("--export-openapi"))
{
    var output = args[Array.IndexOf(args, "--export-openapi") + 1];
    app.Services.SaveSwaggerJson(output);
    return;
}

if (app.Environment.IsDevelopment())
{
    app.UseSwagger();
    app.UseSwaggerUI();
    app.Services.SaveSwaggerJson();
}

// Using reverse proxy
// app.UseHttpsRedirection();

app.UseAuthentication();
app.UseAuthorization();

app.MapControllers();
app.MapMetrics();
app.Run();

// Exposed so integration tests can boot the app via WebApplicationFactory<Program>.
public partial class Program { }
