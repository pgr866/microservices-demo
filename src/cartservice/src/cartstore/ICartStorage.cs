using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace cartservice.cartstore
{
    /// <summary>
    /// Where the carts are kept: one entry per cart, with a field per product holding its quantity.
    /// </summary>
    public interface ICartStorage
    {
        /// <summary>
        /// Adds <paramref name="quantity"/> to the field in one atomic step, creating the cart or the
        /// field if needed, and keeps the cart until <paramref name="expiry"/> after this call.
        /// </summary>
        Task IncrementAsync(string key, string field, long quantity, TimeSpan expiry);

        /// <summary>The fields of the cart, in the order they were added; none if there is no cart.</summary>
        Task<IReadOnlyList<KeyValuePair<string, long>>> GetAllAsync(string key);

        Task DeleteAsync(string key);
    }
}
