package com.snowflake.carbon;

import io.swagger.v3.oas.annotations.OpenAPIDefinition;
import io.swagger.v3.oas.annotations.info.Contact;
import io.swagger.v3.oas.annotations.info.Info;
import io.swagger.v3.oas.annotations.info.License;
import io.swagger.v3.oas.annotations.servers.Server;
import io.swagger.v3.oas.annotations.enums.SecuritySchemeType;
import io.swagger.v3.oas.annotations.security.SecurityScheme;
import org.springframework.amqp.rabbit.annotation.EnableRabbit;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
@EnableRabbit
@SecurityScheme(name = "Bearer", type = SecuritySchemeType.HTTP, scheme = "bearer", bearerFormat = "JWT")
@OpenAPIDefinition(
        info = @Info(
                title = "Snowflake API Reference",
                version = "1.0.0",
                description = "The Snowflake API is organized around REST. This API has predictable resource-oriented URLs, accepts JSON-encoded request bodies, returns JSON-encoded responses, and uses standard HTTP response codes, authentication, and verbs.",
                contact = @Contact(
                        name = "GitHub",
                        url = "https://github.com/getsnowflake/snowflake/issues"
                ),
                license = @License(
                        name = "GNU General Public License v3.0",
                        url = "https://www.gnu.org/licenses/gpl-3.0"
                )
        ),
        servers = {
                @Server(url = "https://snowflake.snowye.dev", description = "Snowflake API Server")
        }
)
public class CarbonApplication {
    public static void main(String[] args) {
        SpringApplication.run(CarbonApplication.class, args);
    }
}
